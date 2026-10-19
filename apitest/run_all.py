"""One command, current URL/method scripts, isolated assertions and local HTML."""
import argparse
import html
import importlib.util
import json
import os
from pathlib import Path
import re
import sys
import time
from datetime import datetime,timezone

ROOT=Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT))

def redact(value):
    text=str(value)
    for key,value in os.environ.items():
        if value and len(value)>3 and any(x in key for x in ['PASSWORD','TOKEN','API_KEY','DSN']):text=text.replace(value,'[redacted]')
    return re.sub(r'eyJ[\w-]+\.[\w-]+\.[\w-]+','[redacted JWT]',text)

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--inject-failure',action='store_true',help='Verify failure exit/report locally');args=parser.parse_args()
    out=ROOT/'reports'/'apitest'/(datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ')+'-'+os.urandom(4).hex())
    out.mkdir(parents=True,exist_ok=True)
    rows=[];context=None
    try:
        if args.inject_failure:raise RuntimeError('Expected injected runner failure')
        if os.environ.get('APITEST_SUITE')=='week6':
            from apitest.week6_support import Context
        else:
            from apitest.support import Context
        context=Context();context.bootstrap()
        cases=[]
        for file in (ROOT/'apitest'/'cases').rglob('*.py'):
            spec=importlib.util.spec_from_file_location('case_'+str(len(cases)),file);module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
            path='/'+str(file.parent.relative_to(ROOT/'apitest'/'cases')).replace('\\','/')
            if path=='/swagger':path+='/'
            cases.append((module.ORDER,path,file.stem,module))
        spec=context.call('GET','/openapi.json')
        documented={(method.upper(),path) for path,item in spec['paths'].items() for method in item if method in ['get','post','put','delete','patch','head','options']}
        assert {(method,path) for _,path,method,_ in cases}==documented,'URL/method cases differ from actual generated contract'
        for _,path,method,module in sorted(cases,key=lambda x:(x[0],x[1],x[2])):
            start=time.monotonic();context.assertions=[];context.timings={};passed=True;failure=''
            try:module.run(context)
            except Exception as error:passed=False;failure=redact(str(error))
            finally:context.reset()
            rows.append({'interface':method+' '+path,'passed':passed,'seconds':round(time.monotonic()-start,3),'assertions':context.assertions,'error':failure,'timing':context.timings})
            print(('PASS' if passed else 'FAIL'),method,path,failure,flush=True)
        # Required fresh Go isolation evidence covers transactional failure/crash
        # scenarios inaccessible via public APIs. No historical or Python substitute.
        evidence=Path(os.environ['APITEST_GO_EVIDENCE'])
        go=json.loads(evidence.read_text(encoding='utf-8-sig'))
        stamp=datetime.fromisoformat(go['utc'].replace('Z','+00:00'))
        assert (datetime.now(timezone.utc)-stamp).total_seconds()<7200 and go['controlled_tests_passed'] and go['exit_code']==0,'Fresh Go fault regression evidence required'
        rows.append({'interface':'supplement: current Go MySQL/Qdrant fault regression','passed':True,'seconds':0,'assertions':['initial transaction/prefix/terminal persistence faults','timeout, cancellation, crash recovery and completed done-loss','UTF-8, history budget, privacy, reindex snapshots and repeated turns'],'error':'','timing':{}})
    except Exception as error:
        rows.append({'interface':'environment/dependency gate','passed':False,'seconds':0,'assertions':[],'error':redact(str(error)),'timing':{}})
    finally:
        if context:context.close()
    passed=bool(rows) and all(row['passed'] for row in rows)
    week6=os.environ.get('APITEST_SUITE')=='week6'
    for row in rows: row['status']='passed' if row['passed'] else 'failed'
    report={'utc':datetime.now(timezone.utc).isoformat(),'passed':passed,'real_model_verified':False,'real_model_status':('Not executed: user-approved Week6 project adjustment uses controlled HTTP models and automated review; no real peer participation' if week6 else 'Not executed; 5.3 requires paid-model authorization, 5.4 genuine peer evidence pending'),'official_compatibility':('Week6 eight MCP operations plus current Week5/Week4 compatibility; original real-peer requirement remains unfulfilled' if week6 else 'Current Week5 contract; Week4 preserved; controlled models only'),'cases':rows}
    (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2),encoding='utf-8')
    body=''.join('<tr><td>'+html.escape(row['interface'])+'</td><td>'+('PASS' if row['passed'] else 'FAIL')+'</td><td>'+str(row['seconds'])+'</td><td>'+html.escape('; '.join(row['assertions']))+'</td><td>'+html.escape(row['error'])+'</td><td>'+html.escape(json.dumps(row['timing']))+'</td></tr>' for row in rows)
    (out/'index.html').write_text('<!doctype html><html lang="zh-CN"><meta charset="utf-8"><title>CampusClaw API验收</title><h1>'+('通过' if passed else '失败/阻塞')+'</h1><p>'+html.escape(report['official_compatibility'])+'</p><p>'+report['real_model_status']+'</p><table border="1"><thead><tr><th>URL/方法</th><th>结果</th><th>秒</th><th>内容与DB断言</th><th>失败原因</th><th>SSE时间</th></tr></thead><tbody>'+body+'</tbody></table></html>',encoding='utf-8')
    Path(os.environ.get('APITEST_RESULT_POINTER',str(out/'pointer.json'))).write_text(json.dumps({'report':str(out/'result.json'),'passed':passed}),encoding='utf-8')
    print(('PASS' if passed else 'FAIL'),sum(r['passed'] for r in rows),'/',len(rows),'report:',out.relative_to(ROOT)/'index.html')
    return 0 if passed else 1

if __name__=='__main__':sys.exit(main())

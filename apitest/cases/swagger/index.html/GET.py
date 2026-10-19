from pathlib import Path
import sys,json,os
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=51
def run(c):
    page=c.call('GET','/swagger/index.html')
    c.check(b'/swagger/vendor/swagger-ui-bundle.js' in page and b'stream-debug' in page,'local UI and real SSE debug panel')
    for path in ['/swagger/vendor/swagger-ui-bundle.js','/swagger/vendor/swagger-ui.css','/swagger/init.js','/swagger/sse.js']:
        c.check(len(c.call('GET',path))>0,'Swagger asset available')
    evidence=json.loads(Path(os.environ['APITEST_SWAGGER_EVIDENCE']).read_text(encoding='utf-8-sig'))
    c.check(len(evidence['checks'])>=9 and any('actual Swagger Execute' in v for v in evidence['checks']) and evidence.get('dod',{}).get('D8',{}).get('passed'),'fresh real Swagger browser execution evidence')
if __name__=='__main__':standalone(run)

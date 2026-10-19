from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=11
def run(c):
    count=c.sql('SELECT COUNT(*) AS n FROM materials')[0]['n']
    c.call('POST','/api/materials','a',expected=403)
    c.call('POST','/api/materials',expected=401)
    c.call('POST','/api/materials','ta',expected=400)
    c.check(c.sql('SELECT COUNT(*) AS n FROM materials')[0]['n']==count,'rejected uploads create no DB rows')
    c.control(embedding_failure=True)
    uploaded=c.upload('ta','星河索引失败仍保留原文 '+c.namespace)
    c.check(uploaded['index_status']=='failed','index failure remains201')
    row=c.sql('SELECT plain_text,index_status FROM knowledge_entries WHERE material_id=%s',(uploaded['material_id'],))[0]
    c.check(row['index_status']=='failed' and '索引失败' in row['plain_text'],'committed source survives index failure')
    c.check(c.call('GET','/api/materials/'+uploaded['material_id'],'a')['plain_text']==row['plain_text'],'failed index source still readable')
if __name__=='__main__':standalone(run)

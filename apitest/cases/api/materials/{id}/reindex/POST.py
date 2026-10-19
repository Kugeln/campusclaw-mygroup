from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=14
def run(c):
    path='/api/materials/'+c.materials['a']+'/reindex'
    before=c.sql('SELECT id,chunk_text FROM knowledge_chunks WHERE material_id=%s ORDER BY chunk_index',(c.materials['a'],))
    c.call('POST',path,'a',{},expected=403);c.call('POST',path,'tb',{},expected=404)
    c.call('POST',path,'ta',{'size':0},expected=400)
    c.check(c.sql('SELECT id,chunk_text FROM knowledge_chunks WHERE material_id=%s ORDER BY chunk_index',(c.materials['a'],))==before,'invalid strategy leaves old index untouched')
    value=c.call('POST',path,'ta',{'size':400,'overlap':40})
    current=c.sql('SELECT id,index_status FROM knowledge_chunks WHERE material_id=%s',(c.materials['a'],))
    c.check(value['index_status']=='ready' and current and all(r['index_status']=='ready' for r in current),'new chunks published ready')
    c.check(not ({r['id'] for r in before}&{r['id'] for r in current}),'reindex replaces UUIDs without reusing references')
if __name__=='__main__':standalone(run)

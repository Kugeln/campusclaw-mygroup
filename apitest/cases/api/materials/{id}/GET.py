from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=12
def run(c):
    for group,role,foreign in [('a','a','b'),('b','b','a')]:
        path='/api/materials/'+c.materials[group]
        value=c.call('GET',path,role)
        text=c.sql('SELECT plain_text FROM knowledge_entries WHERE material_id=%s',(c.materials[group],))[0]['plain_text']
        c.check(value['plain_text']==text==c.texts[group],'material text matches DB fixture')
        c.call('GET',path,foreign,expected=404)
    c.call('GET','/api/materials/00000000-0000-4000-8000-000000000000','a',expected=404)
if __name__=='__main__':standalone(run)

from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=10
def run(c):
    for role,group in [('a',1),('b',2)]:
        value=c.call('GET','/api/materials',role)['items']
        ids={r['id'] for r in c.sql('SELECT id FROM materials WHERE class_id=%s',(group,))}
        c.check({r['id'] for r in value}==ids,'material list exactly matches current class DB')
        c.call('GET','/api/materials?class_id=2',role,expected=400)
    c.call('GET','/api/materials',expected=401)
if __name__=='__main__':standalone(run)

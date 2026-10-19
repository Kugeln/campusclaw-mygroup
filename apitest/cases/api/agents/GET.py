from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=30
def run(c):
    for role,group in [('ta',1),('a',1),('tb',2),('b',2)]:
        rows=c.call('GET','/api/agents',role)['agents']
        expected={r['id'] for r in c.sql('SELECT id FROM assistants WHERE class_id=%s'+(' AND enabled=1' if role in ['a','b'] else ''),(group,))}
        c.check({r['id'] for r in rows}==expected,'teacher/student assistant visibility equals DB')
    c.call('GET','/api/agents',expected=401)
if __name__=='__main__':standalone(run)

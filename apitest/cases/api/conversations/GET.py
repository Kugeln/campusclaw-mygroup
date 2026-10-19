from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=40
def run(c):
    for role,group in [('a',1),('a2',1),('b',2)]:
        rows=c.call('GET','/api/conversations',role)['conversations']
        expected={r['id'] for r in c.sql('SELECT id FROM conversations WHERE user_id=%s AND class_id=%s',(c.users[role],group))}
        c.check({r['id'] for r in rows}==expected and all(r['user_id']==c.users[role] for r in rows),'private list equals owner/class DB set')
    c.call('GET','/api/conversations','ta',expected=403);c.call('GET','/api/conversations',expected=401)
if __name__=='__main__':standalone(run)

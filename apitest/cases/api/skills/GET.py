from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=34
def run(c):
    before=c.sql('SELECT COUNT(*) AS n FROM skills')[0]['n']
    for role,group in [('a',1),('b',2)]:
        skills=c.call('GET','/api/skills',role)['skills']
        expected=c.sql('SELECT id FROM skills WHERE class_id=%s',(group,))
        c.check({s['id'] for s in skills}=={s['id'] for s in expected},'current class skill list equals DB')
        c.check(all(s['source'] and s['trigger'] for s in skills),'parsed source metadata available')
    c.check(c.sql('SELECT COUNT(*) AS n FROM skills')[0]['n']==before,'skill GET never provisions rows')
if __name__=='__main__':standalone(run)

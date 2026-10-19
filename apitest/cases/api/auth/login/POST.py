from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=1
def run(c):
    before=c.sql('SELECT id,session_version FROM users ORDER BY id')
    for role in c.usernames:
        result=c.login(role)
        c.check(result['token_type']=='Bearer' and result['expires_in']==28800 and result['redirect']=='/materials','8h Bearer login response')
    c.call('POST','/api/auth/login',value={'username':c.usernames['a'],'password':'wrong'},expected=401)
    c.call('POST','/api/auth/login',value={'username':'missing-'+c.namespace,'password':c.password},expected=401)
    c.check(c.sql('SELECT id,session_version FROM users ORDER BY id')==before,'login does not reset versions')
if __name__=='__main__':standalone(run)

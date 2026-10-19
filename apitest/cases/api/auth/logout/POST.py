from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=100
def run(c):
    old=c.credentials['a'];second=c.login('a')['access_token']
    before=c.sql('SELECT session_version FROM users WHERE id=%s',(c.users['a'],))[0]['session_version']
    c.check(c.call('POST','/api/auth/logout','a',expected=204) is None,'logout204 empty body')
    for token in [old,second]:c.call('GET','/api/auth/me',token,expected=401)
    after=c.sql('SELECT session_version FROM users WHERE id=%s',(c.users['a'],))[0]['session_version']
    c.check(after==before+1,'all-device revocation persisted')
    c.login('a');c.call('GET','/api/auth/me','a')
if __name__=='__main__':standalone(run)

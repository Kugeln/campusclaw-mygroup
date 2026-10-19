from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=2
def run(c):
    for role in c.usernames:
        me=c.call('GET','/api/auth/me',role)
        row=c.sql('SELECT id,class_id,role FROM users WHERE id=%s',(me['user_id'],))[0]
        c.check(me['class_id']==row['class_id'] and me['role']==row['role'],'identity matches current MySQL')
    c.call('GET','/api/auth/me',expected=401)
    c.call('GET','/api/auth/me','invalid',expected=401)
    c.call('GET','/api/auth/me','Bearer invalid',expected=401)
    c.check(c.call('GET','/api/auth/me?class_id=2&role=teacher','a')['class_id']==1,'client scope does not override identity')
if __name__=='__main__':standalone(run)

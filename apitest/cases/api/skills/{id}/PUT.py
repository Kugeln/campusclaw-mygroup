from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=35
def run(c):
    path='/api/skills/'+str(c.skills['a']);before=c.sql('SELECT enabled,revision FROM skills WHERE id=%s',(c.skills['a'],))[0]
    c.call('PUT',path,'a',{'enabled':False},expected=403);c.call('PUT',path,'tb',{'enabled':False},expected=404)
    c.call('PUT',path,'ta',{'enabled':'false'},expected=400);c.call('PUT',path,'ta',{'source':'arbitrary'},expected=400)
    c.check(c.sql('SELECT enabled,revision FROM skills WHERE id=%s',(c.skills['a'],))[0]==before,'rejected skill updates unchanged')
    for flag in [False,True]:
        value=c.call('PUT',path,'ta',{'enabled':flag});row=c.sql('SELECT enabled,revision FROM skills WHERE id=%s',(c.skills['a'],))[0]
        c.check(bool(row['enabled'])==flag and row['revision']==value['revision'],'toggle state/version persisted')
if __name__=='__main__':standalone(run)

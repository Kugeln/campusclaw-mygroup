from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=42
def run(c):
    id=c.conversations['a'];path='/api/conversations/'+str(id)
    before=c.sql('SELECT COUNT(*) AS n FROM conversation_messages')[0]['n']
    value=c.call('GET',path,'a');c.check(value['messages']==[] and value['conversation']['user_id']==c.users['a'],'own empty history')
    for role in ['a2','b']:c.call('GET',path,role,expected=404)
    c.call('GET',path,'ta',expected=403)
    c.check(c.sql('SELECT COUNT(*) AS n FROM conversation_messages')[0]['n']==before,'private history denial has no write')
if __name__=='__main__':standalone(run)

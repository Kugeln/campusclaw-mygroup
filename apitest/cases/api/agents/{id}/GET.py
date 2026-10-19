from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=32
def run(c):
    path='/api/agents/'+str(c.agents['a'])
    value=c.call('GET',path,'a');row=c.sql('SELECT prompt,revision FROM assistants WHERE id=%s',(value['id'],))[0]
    c.check(value['prompt']==row['prompt'] and value['revision']==row['revision'],'agent read matches saved version')
    c.call('GET',path,'b',expected=404)
    c.call('GET','/api/agents/0','a',expected=400)
if __name__=='__main__':standalone(run)

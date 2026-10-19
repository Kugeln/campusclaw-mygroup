from apitest.support import standalone
ORDER=35.1
def run(c):
    path='/api/skills/'+str(c.crud_skill)
    c.call('DELETE',path,'a',expected=403)
    c.call('DELETE',path,'tb',expected=404)
    agent=c.call('POST','/api/agents','ta',c.agent_input(name=c.namespace+'-multi',skill_ids=[c.skills['a'],c.crud_skill]),expected=201)
    c.call('DELETE',path,'ta',expected=204)
    c.call('GET',path,'ta',expected=404)
    c.check(c.call('GET','/api/agents/'+str(agent['id']),'ta')['skill_ids']==[c.skills['a']],'delete removes only current binding')
if __name__=='__main__':standalone(run)

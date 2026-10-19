from apitest.support import standalone
ORDER=34.2
def run(c):
    path='/api/skills/'+str(c.crud_skill)
    source=c.call('GET',path,'ta')['source']
    c.check(c.call('GET',path,'a')['source']==source,'authenticated class source read')
    c.call('GET',path,'tb',expected=404)
    c.call('GET',path,expected=401)
    edited=source.replace('理解问题','逐步检查问题')
    updated=c.call('PUT',path,'ta',{'source':edited})
    c.check(updated['source']==edited and c.call('GET',path,'ta')['source']==edited,'source update persisted')
    c.call('PUT',path,'ta',{'source':source,'enabled':True},expected=400)
if __name__=='__main__':standalone(run)

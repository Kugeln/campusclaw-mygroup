from apitest.support import standalone
ORDER=34.1
def run(c):
    source=c.skill_source(name=c.namespace+'-CRUD')
    before=c.sql('SELECT COUNT(*) AS n FROM skills')[0]['n']
    c.call('POST','/api/skills','a',{'source':source},expected=403)
    c.call('POST','/api/skills',value={'source':source},expected=401)
    c.call('POST','/api/skills','ta',{'source':'错误'},expected=400)
    c.check(c.sql('SELECT COUNT(*) AS n FROM skills')[0]['n']==before,'rejected sources have no writes')
    created=c.call('POST','/api/skills','ta',{'source':source},expected=201)
    c.crud_skill=created['id']
    c.check(created['source']==source,'source round trip')
    c.call('POST','/api/skills','ta',{'source':source},expected=400)
if __name__=='__main__':standalone(run)

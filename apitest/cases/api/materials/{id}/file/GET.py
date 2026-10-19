from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=13
def run(c):
    for group,role,foreign in [('a','a','b'),('b','b','a')]:
        path='/api/materials/'+c.materials[group]+'/file'
        c.check(c.call('GET',path,role)==c.texts[group].encode(),'attachment bytes equal relational source')
        c.call('GET',path,foreign,expected=404)
        c.call('GET',path,expected=401)
    c.call('GET','/uploads/files/'+c.materials['a'],expected=404)
if __name__=='__main__':standalone(run)

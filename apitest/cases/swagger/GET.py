from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=52
def run(c):
    c.check(c.call('GET','/swagger/')==c.call('GET','/swagger/index.html'),'Swagger alias serves identical page')
    c.call('GET','/api/conversations',expected=401)
if __name__=='__main__':standalone(run)

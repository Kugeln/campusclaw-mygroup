from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=20
def run(c):
    for role,word,foreign,group in [('a','星河','海风',1),('b','海风','星河',2)]:
        for mode in ['keyword','vector','hybrid']:
            value=c.call('GET','/api/search?q='+word+'&mode='+mode,role)
            c.check(value['mode']==mode and value['items'],'three-mode search content')
            for s in value['items']:
                row=c.sql('SELECT class_id,chunk_text FROM knowledge_chunks WHERE id=%s',(s['chunk_id'],))[0]
                c.check(row['class_id']==group and row['chunk_text']==s['chunk_text'],'search provenance and class match MySQL')
            c.check(c.call('GET','/api/search?q='+foreign+'&mode='+mode,role)['items']==[],'bidirectional foreign-only query empty')
    before=c.control()['embeddings'];c.call('GET','/api/search?q=星河&mode=keyword','a')
    c.check(c.control()['embeddings']==before,'keyword zero embedding calls')
    c.control(embedding_failure=True)
    c.call('GET','/api/search?q=星河&mode=keyword','a');c.call('GET','/api/search?q=星河&mode=hybrid','a',expected=503)
    c.call('GET','/api/search?q=星河&mode=invalid','a',expected=400)
if __name__=='__main__':standalone(run)

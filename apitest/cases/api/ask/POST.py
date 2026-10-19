from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=21
def run(c):
    before=c.sql('SELECT COUNT(*) AS n FROM conversation_messages')[0]['n']
    value=c.call('POST','/api/ask','a',{'question':'星河','mode':'hybrid'})
    c.check('[1]' in value['answer'] and value['sources'],'single answer citations')
    for s in value['sources']:
        c.check(c.sql('SELECT chunk_text FROM knowledge_chunks WHERE id=%s',(s['chunk_id'],))[0]['chunk_text']==s['chunk_text'],'ask sources match DB')
    calls=c.control()['dialogues'];empty=c.call('POST','/api/ask','a',{'question':'完全不存在','mode':'hybrid'})
    c.check(empty=={'answer':'资料中未找到相关内容','sources':[]} and c.control()['dialogues']==calls,'ask no evidence exact text and zero model calls')
    c.control(bad_answer=True);c.call('POST','/api/ask','a',{'question':'星河','mode':'hybrid'},expected=502)
    c.check(c.sql('SELECT COUNT(*) AS n FROM conversation_messages')[0]['n']==before,'single ask never creates conversation messages')
    c.call('POST','/api/ask','a',{'q':'星河'},expected=400)
if __name__=='__main__':standalone(run)

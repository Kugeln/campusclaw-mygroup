from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=50
def run(c):
    value=c.call('GET','/openapi.json')
    c.check(value['openapi']=='3.0.3','OpenAPI 3.0.3; route set dynamically checked by runner')
    c.check(value['components']['securitySchemes']['bearerAuth']['scheme']=='bearer','Bearer declaration')
    c.check('get' in value['paths']['/api/search'] and 'post' not in value['paths']['/api/search'],'real GET search contract')
    c.check('text/event-stream' in value['paths']['/api/conversations/{id}/messages']['post']['responses']['200']['content'],'SSE content explicitly documented')
    c.check('/api/login' not in value['paths'],'no invented official login alias')
if __name__=='__main__':standalone(run)

from pathlib import Path
import sys
sys.path.insert(0,str(next(p for p in Path(__file__).resolve().parents if (p/'apitest/run_all.py').is_file())))
from apitest.support import standalone
ORDER=67

def run(c):
    c.mcp_case('delete')

if __name__=='__main__':standalone(run)

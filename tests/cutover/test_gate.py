import unittest
from gate import allow_cutover


class GateTests(unittest.TestCase):
    def test_only_verified_stopped_source_and_healthy_target(self):
        report={'Verified':True,'Mode':'migrate','SourceSHA256':'source','Fingerprint':'rows',
                'Tables':{str(i):{} for i in range(7)}}
        self.assertTrue(allow_cutover(report,'source','rows',True,True))
        for changed in ({'Verified':False},{'Mode':'preflight'},{'SourceSHA256':'stale'},
                        {'Fingerprint':'different'},{'Tables':{}}):
            self.assertFalse(allow_cutover(dict(report,**changed),'source','rows',True,True))
        self.assertFalse(allow_cutover(None,'source','rows',True,True))
        self.assertFalse(allow_cutover(report,'source','rows',False,True))
        self.assertFalse(allow_cutover(report,'source','rows',True,False))


if __name__=='__main__':
    unittest.main()

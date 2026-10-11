"""Tests for comparison rules, not substitutes for either live HTTP backend."""
import sys
from pathlib import Path
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parent))
from adapters import Response, HTTPAdapter
from cases import attachment
from cases import Case
from run import select_cases
from accepted_differences import RULES, expected_status, assert_response, expected_allow


class HarnessTests(unittest.TestCase):
    def test_approved_rules_are_exact_and_do_not_cover_fixed_errors(self):
        self.assertEqual(len(RULES),29)
        for name in ('multipart.truncated','upload.bad_extension_then_identity','upload.invalid_text_then_second_file','method.OPTIONS./api/ask'):
            self.assertNotIn(name,RULES)
        self.assertEqual(expected_status('upload.over_limit','Go',413),413)
        self.assertEqual(expected_status('upload.oversize_then_identity','Go',400),413)
        self.assertEqual(expected_allow('/api/materials','Go'),['GET','HEAD','POST'])

    def test_approved_json_rule_rejects_changed_fields_or_media(self):
        headers=[('Content-Type','application/json'),('Cache-Control','no-store'),('X-Content-Type-Options','nosniff')]
        good=b'{"error":"Client identity fields are not accepted"}'
        assert_response('multipart.large_form_field','Go',Response(400,headers,good))
        for body in (b'{"error":"another error"}',b'{"error":"Client identity fields are not accepted","extra":1}'):
            with self.assertRaises(AssertionError):
                assert_response('multipart.large_form_field','Go',Response(400,headers,body))
        with self.assertRaises(AssertionError):
            assert_response('multipart.large_form_field','Go',Response(400,[('Content-Type','text/html')],good))

    def test_exact_case_selection_preserves_functions(self):
        cases = [Case('a', lambda:None), Case('b', lambda:None)]
        self.assertEqual(select_cases(cases,[]),cases)
        self.assertEqual(select_cases(cases,['b']),[cases[1]])
        self.assertIs(select_cases(cases,['b'])[0].run,cases[1].run)
        with self.assertRaises(ValueError):
            select_cases(cases,['absent'])

    def test_rfc_filename_star_over_ascii_fallback(self):
        for value in (
            "attachment; filename=.md; filename*=UTF-8''%E4%B8%AD%E6%96%87.md",
            "attachment; filename*=UTF-8''%E4%B8%AD%E6%96%87.md",
        ):
            response = Response(200,[('Content-Disposition',value)],b'')
            self.assertEqual(attachment(response),{'kind':'attachment','filename':'中文.md'})

    def test_no_demo_or_remote_adapter(self):
        for url in ('http://127.0.0.1:8080','http://127.0.0.1','http://example.com:45678'):
            with self.assertRaises(RuntimeError):
                HTTPAdapter('test',url,'synthetic','synthetic','unused',None)

    def test_summary_keeps_json_error_and_redacts_token(self):
        response=Response(401,[('Content-Type','application/json')],b'{"error":"denied"}')
        self.assertEqual(response.summary(),{'status':401,'media_type':'application/json','json':{'error':'denied'}})
        response=Response(200,[('Content-Type','application/json')],b'{"access_token":"must-not-appear","expires_in":28800}')
        self.assertEqual(response.summary()['json'],{'access_token':'[redacted]','expires_in':28800})


if __name__=='__main__':
    unittest.main()

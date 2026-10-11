"""Fail-closed rehearsal gate; no service/deployment side effects here."""


def allow_cutover(report, source_sha, fingerprint, source_stopped, app_healthy):
    return bool(source_stopped and app_healthy and report
                and report.get('Verified') is True
                and report.get('Mode') == 'migrate'
                and report.get('SourceSHA256') == source_sha
                and report.get('Fingerprint') == fingerprint
                and len(report.get('Tables', {})) == 7)

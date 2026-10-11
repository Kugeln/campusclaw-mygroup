"""Run only retrieval tests in an isolated short-lived Compose project."""
import os
from pathlib import Path
import secrets
import subprocess
import sys

root = Path(__file__).resolve().parents[1]
env = os.environ.copy()
env['RETRIEVAL_MYSQL_PASSWORD'] = secrets.token_urlsafe(24)
env['RETRIEVAL_MYSQL_ROOT_PASSWORD'] = secrets.token_urlsafe(24)
project = 'campusclaw-retrieval-check-' + secrets.token_hex(4)
compose = ['docker', 'compose', '-f', str(root / 'docker-compose.retrieval.yml'), '-p', project]


def run(args):
    subprocess.run(compose + args, cwd=root, env=env, check=True)


if __name__ == '__main__':
    try:
        run(['up', '-d', '--wait', 'mysql', 'qdrant'])
        if '--no-build' not in sys.argv:
            run(['build', 'app'])
        run(['run', '--rm', '--no-deps', '-T', '-e', 'RETRIEVAL_INTEGRATION=1',
             '-e', 'RETRIEVAL_TEST_PROJECT=' + project,
             '-v', str(root) + ':/workspace:ro', '-w', '/workspace', '--entrypoint', 'python',
             'app', '-B', '-m', 'pytest', '-p', 'no:cacheprovider', 'tests/retrieval', '-q'])
    finally:
        # Only the freshly generated test project; never the classroom demo volumes.
        run(['down', '-v', '--remove-orphans'])

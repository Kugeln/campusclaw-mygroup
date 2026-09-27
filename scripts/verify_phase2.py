"""Real MySQL tests in a disposable network/database; never mount user data."""
import os
from pathlib import Path
import secrets
import subprocess
import time

ROOT = Path(__file__).resolve().parents[1]


def main():
    name = 'campusclaw-phase2-test-' + secrets.token_hex(4)
    env = os.environ.copy()
    env.update(DB_PASSWORD=secrets.token_hex(24), MYSQL_ROOT_PASSWORD=secrets.token_hex(24),
               DB_HOST=name, DB_PORT='3306', DB_USER='campusclaw', DB_NAME='campusclaw',
               MYSQL_DATABASE='campusclaw', MYSQL_USER='campusclaw', UPLOAD_ROOT='/tmp/unused', PHASE2_ISOLATED='1')
    env['MYSQL_PASSWORD'] = env['DB_PASSWORD']
    def run(args, check=True, timeout=60):
        started = time.monotonic()
        with subprocess.Popen(['docker', *args], cwd=ROOT, env=env,
                              stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True) as process:
            while True:
                try:
                    stdout, stderr = process.communicate(timeout=15)
                    break
                except subprocess.TimeoutExpired:
                    print(f'Waiting for docker {args[0]}: {int(time.monotonic()-started)}s', flush=True)
                    if time.monotonic()-started >= timeout:
                        process.kill()
                        process.communicate()
                        raise RuntimeError(f'docker {args[0]} exceeded {timeout}s') from None
        result = subprocess.CompletedProcess(args, process.returncode, stdout, stderr)
        output = result.stdout + result.stderr
        if any(env[key] in output for key in ('DB_PASSWORD', 'MYSQL_ROOT_PASSWORD')):
            raise RuntimeError('Credential in output (suppressed)')
        if check and result.returncode:
            print(output)
            raise RuntimeError('Isolated verification failed')
        return result
    try:
        run(['network', 'create', name])
        run(['run', '-d', '--name', name, '--network', name, '--tmpfs', '/var/lib/mysql',
             '-e', 'MYSQL_DATABASE', '-e', 'MYSQL_USER', '-e', 'MYSQL_PASSWORD', '-e', 'MYSQL_ROOT_PASSWORD',
             'mysql:8.4.4', '--max-allowed-packet=67108864', '--log-bin-trust-function-creators=1'])
        for _ in range(120):
            probe = run(['exec', name, 'sh', '-c', 'MYSQL_PWD="$MYSQL_PASSWORD" mysql --protocol=TCP -h127.0.0.1 -u"$MYSQL_USER" -D"$MYSQL_DATABASE" -e "SELECT 1"'], check=False)
            if probe.returncode == 0:
                break
            time.sleep(1)
        else:
            raise RuntimeError('Temporary MySQL not ready')
        print('PASS isolated MySQL ready', flush=True)
        args = ['run', '--rm', '--name', name+'-go', '--network', name,
                '--mount', f'type=bind,source={ROOT},target=/src,readonly', '-w', '/src']
        for key in ('DB_HOST', 'DB_PORT', 'DB_USER', 'DB_NAME', 'DB_PASSWORD', 'UPLOAD_ROOT', 'PHASE2_ISOLATED'):
            args.extend(['-e', key])
        args.extend(['golang:1.26.0-alpine3.23', 'sh', '-c',
                     'go test -mod=readonly -p=1 -timeout=120s -count=1 -v -tags integration ./... && go build -mod=readonly -o /tmp/admin ./cmd/admin && go build -mod=readonly -o /tmp/migrate ./cmd/migrate && go build -mod=readonly -o /tmp/server ./cmd/server && go list -deps ./cmd/server > /tmp/server-deps && ! grep -E "modernc.org|internal/migration" /tmp/server-deps'])
        print('Running Phase 2 integration and builds (600s deadline)', flush=True)
        result = run(args, timeout=600)
        print(result.stdout, flush=True)
        print('PASS phase2 builds and real MySQL integration', flush=True)
    finally:
        cleanup = [run(['rm', '-f', name+'-go'], check=False),
                   run(['rm', '-f', name], check=False),
                   run(['network', 'rm', name], check=False)]
        # --rm may already have removed the Go runner; verify actual residue.
        residue = run(['ps', '-a', '--filter', 'name='+name, '--format', '{{.Names}}']).stdout.strip()
        if residue or cleanup[-1].returncode:
            raise RuntimeError('Cleanup incomplete; inspect isolated project ' + name)
        print('Temporary Phase 2 resources cleaned: ' + name, flush=True)


if __name__ == '__main__':
    main()

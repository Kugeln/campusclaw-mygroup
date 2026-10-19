"""Backup/restore rehearsal on the new align-storage server only. Never DROP."""
import hashlib
import json
from pathlib import Path
import re
import secrets
import shutil
import subprocess

ROOT = Path(__file__).resolve().parents[2]


def run(args, **kwargs):
    result = subprocess.run(args, cwd=ROOT, stdout=subprocess.PIPE, stderr=subprocess.PIPE, **kwargs)
    if result.returncode:
        raise RuntimeError(f"isolated recovery command failed: {args[0]} exit={result.returncode}")
    return result.stdout


def main():
    connection = json.loads((ROOT / 'runtime/align-storage-connection.json').read_text(encoding='utf-8-sig'))
    container = connection['container']
    if not re.fullmatch(r'campusclaw-align-storage-[a-f0-9]{8}', container):
        raise RuntimeError('invalid recovery server')
    info = json.loads(run(['docker', 'inspect', container]))[0]
    if info['Config']['Labels'].get('campusclaw.phase') != 'align-storage' or not info['State']['Running']:
        raise RuntimeError('new isolated server not running')
    if any(m['Type'] != 'tmpfs' for m in info['Mounts']):
        raise RuntimeError('persistent/foreign mounts forbidden')
    evidence = Path(connection['evidence']).resolve()
    evidence.relative_to(ROOT / 'runtime')
    recovery_run = secrets.token_hex(4)
    evidence = evidence / ('recovery-' + recovery_run)
    evidence.mkdir(exist_ok=False)
    runner = ['powershell', '-NoProfile', '-File', 'tests/go_rag/run_align_storage.ps1', '-RecoveryRun', recovery_run, '-Stage']
    print(run(runner + ['RecoveryPrepare']).decode('utf-8', errors='replace'))
    plan_path = evidence / 'recovery-plan.json'
    plan = json.loads(plan_path.read_text(encoding='utf-8'))
    source = plan['source']
    restored = 'align_w5_' + secrets.token_hex(8)
    if not re.fullmatch(r'align_w5_[a-f0-9]{16}', source) or source == restored:
        raise RuntimeError('unsafe recovery database')
    # Root password is expanded only inside this newly created container. It is
    # never present in CLI arguments, returned dump bytes, stdout, or report.
    backup = run(['docker', 'exec', container, 'sh', '-c',
                  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysqldump -uroot --single-transaction --set-gtid-purged=OFF --skip-add-drop-table --skip-dump-date --no-tablespaces --column-statistics=0 "$1"', '_', source])
    if re.search(rb'\bDROP\s+(?:TABLE|DATABASE)\b', backup, re.I):
        raise RuntimeError('destructive backup statements refused')
    backup_path = evidence / 'v2-before-upgrade.sql'
    backup_path.write_bytes(backup)
    run(['docker', 'exec', container, 'sh', '-c',
         'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -uroot -e "$1"', '_',
         f'CREATE DATABASE {restored} CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin'])
    run(['docker', 'exec', '-i', container, 'sh', '-c',
         'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -uroot "$1"', '_', restored], input=backup)
    destination = evidence / 'recovery-restored-files'
    destination.mkdir(exist_ok=False)
    for name, expected in plan['files'].items():
        if not re.fullmatch(r'[a-f0-9-]{36}', name):
            raise RuntimeError('invalid fixture file')
        original = evidence / 'recovery-source-files' / name
        if hashlib.sha256(original.read_bytes()).hexdigest() != expected:
            raise RuntimeError('fixture file hash mismatch')
        shutil.copy2(original, destination / name)
    plan['restored'] = restored
    plan['backup_sha256'] = hashlib.sha256(backup).hexdigest()
    plan_path.write_text(json.dumps(plan, ensure_ascii=False, indent=2), encoding='utf-8')
    print(run(runner + ['RecoveryFinish']).decode('utf-8', errors='replace'))
    print('Isolated backup, independent restore, forward repair and data-loss boundary verified.')


if __name__ == '__main__':
    main()

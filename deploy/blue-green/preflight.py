#!/usr/bin/env python3
"""首次迁移只读诊断；输出白名单字段，不建立审批状态，也不修改生产配置。"""
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import time

DIRECTORY = Path('/root/proj/sub2api/deploy')
CONTAINERS = ('sub2api', 'sub2api-blue', 'sub2api-green', 'sub2api-postgres', 'sub2api-redis')


def run(*args):
    result = subprocess.run(args, text=True, capture_output=True, timeout=30, check=False)
    if result.returncode:
        # stderr 和完整命令可能带配置/密钥，不回传到公开 Actions 日志。
        raise RuntimeError(f'{args[0]} 只读查询失败（退出码 {result.returncode}）')
    return result.stdout


def container_summary(data):
    config = data['Config']
    env = dict(item.split('=', 1) for item in config.get('Env', []) if '=' in item)
    return {
        'name': data['Name'].lstrip('/'),
        'status': data['State']['Status'],
        'image_id': data['Image'],
        'revision': (config.get('Labels') or {}).get('org.opencontainers.image.revision'),
        'environment_budget': {key: env.get(key) for key in ('DATABASE_MAX_OPEN_CONNS', 'GOMEMLIMIT')},
        'container_memory_limit': data['HostConfig'].get('Memory'),
        'ports': data['NetworkSettings'].get('Ports'),
        'mounts': [{key: mount.get(key) for key in ('Type', 'Source', 'Destination', 'RW')}
                   for mount in data.get('Mounts', [])],
    }


def endpoint_command(name, port):
    host = name+'.saviour.cc.cd'
    # 只信任服务器既有 Origin 证书，不关闭 TLS 校验或修改系统 CA。
    origin = ['--cacert','/root/cert/saviour.cc.cd/saviour.cc.cd.pem'] if name == 'api' else []
    return ['curl','--fail','--silent','--max-time','5','--noproxy','*',*origin,
            '--resolve',f'{host}:{port}:127.0.0.1','-D','-','-o','/dev/null',f'https://{host}:{port}/readyz']


def collect():
    report = {'captured_at': time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime()),
              'read_only': True, 'deployment_approved': False, 'containers': [], 'blockers': []}
    machine_id = Path('/etc/machine-id').read_bytes().strip()
    report['machine_id_sha256'] = hashlib.sha256(machine_id).hexdigest()
    memory = dict(line.split(':', 1) for line in Path('/proc/meminfo').read_text().splitlines())
    report['memory_total_bytes'] = int(memory['MemTotal'].split()[0]) * 1024
    report['memory_available_bytes'] = int(memory['MemAvailable'].split()[0]) * 1024
    report['compose_sha256'] = hashlib.sha256((DIRECTORY/'docker-compose.yml').read_bytes()).hexdigest()
    names = set(run('docker', 'ps', '-a', '--format', '{{.Names}}').splitlines())
    for name in CONTAINERS:
        if name in names:
            report['containers'].append(container_summary(json.loads(run('docker', 'inspect', name))[0]))
    if 'sub2api-postgres' in names:
        report['postgres_max_connections'] = int(run(
            'docker', 'exec', 'sub2api-postgres', 'sh', '-c',
            'exec psql -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-sub2api}" -Atc "SHOW max_connections"').strip())
    else:
        report['blockers'].append('找不到现有 PostgreSQL 容器')
    report['approved_files_present'] = {name: (DIRECTORY/'blue-green'/name).is_file()
                                        for name in ('config.json', 'state.json')}
    report['first_initialization_needed'] = not all(report['approved_files_present'].values())
    report['legacy_present'] = 'sub2api' in names
    # 文件首次不存在是正常状态，由 tag Release 自动发现并建立，不再列为资源阻塞。
    state_path = DIRECTORY/'blue-green/state.json'
    if state_path.is_file():
        state = json.loads(state_path.read_text())
        report['deployment_state'] = {key:state.get(key) for key in
            ('phase','active','pending','pending_reason','switched_at','switch_seconds','resources','verified_instance','legacy_retained','last_error')}
    observation = DIRECTORY/'blue-green/observation.json'
    if observation.is_file():
        report['health_observation'] = json.loads(observation.read_text())
    if 'sub2api-postgres' in names:
        sql = "SELECT json_build_object('used',(SELECT count(*) FROM pg_stat_activity WHERE backend_type='client backend'),'reserved',current_setting('superuser_reserved_connections')::int+coalesce(current_setting('reserved_connections',true),'0')::int)"
        report['postgres_connections'] = json.loads(run('docker','exec','sub2api-postgres','sh','-c',
            'exec psql -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-sub2api}" -Atc "$1"','psql',sql))
    report['endpoint_checks'] = {}
    for name,port in (('api',2503),('direct',443)):
        result = subprocess.run(endpoint_command(name,port),text=True,capture_output=True,timeout=10,check=False)
        report['endpoint_checks'][name] = {'curl_exit':result.returncode, 'headers':[line for line in result.stdout.splitlines() if line.lower().startswith(('http/','x-sub2api-'))]}
    return report


if __name__ == '__main__':
    try:
        print(json.dumps(collect(), ensure_ascii=False, indent=2))
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.TimeoutExpired) as exc:
        print(json.dumps({'read_only': True, 'deployment_approved': False,
                          'error': f'只读诊断未完成：{type(exc).__name__}，未执行部署'}, ensure_ascii=False))
        sys.exit(1)

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


def collect():
    report = {'captured_at': time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime()),
              'read_only': True, 'deployment_approved': False, 'containers': [], 'blockers': []}
    machine_id = Path('/etc/machine-id').read_bytes().strip()
    report['machine_id_sha256'] = hashlib.sha256(machine_id).hexdigest()
    memory = dict(line.split(':', 1) for line in Path('/proc/meminfo').read_text().splitlines())
    report['memory_total_bytes'] = int(memory['MemTotal'].split()[0]) * 1024
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
    if not all(report['approved_files_present'].values()):
        report['blockers'].append('缺少已审定的蓝绿 config.json/state.json；不能直接打 tag 接管')
    if 'sub2api' in names:
        report['blockers'].append('仍存在旧单实例容器；必须另行确认首次迁移和旧连接/用量处理')
    # 环境值不保证覆盖持久配置，因此本报告不把估计预算判为已通过。
    report['blockers'].append('双实例实际生效的资源预算与旧实例排空仍需审定；诊断成功不等于批准部署')
    return report


if __name__ == '__main__':
    try:
        print(json.dumps(collect(), ensure_ascii=False, indent=2))
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.TimeoutExpired) as exc:
        print(json.dumps({'read_only': True, 'deployment_approved': False,
                          'error': f'只读诊断未完成：{type(exc).__name__}，未执行部署'}, ensure_ascii=False))
        sys.exit(1)

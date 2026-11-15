import { pods } from './mockCluster'

/**
 * Output of one command run in a mock container: canned answers for a few
 * common commands, "command not found" for anything else. Used by the
 * one-shot exec and by the mock terminal.
 */
export function mockExecOutput(ns: string, podName: string, container: string, command: string): string[] {
  const pod = pods.find((p) => p.namespace === ns && p.name === podName)
  const env = pod?.containers[0]?.env ?? []
  const [bin, ...args] = command.trim().split(/\s+/)
  switch (bin) {
    case '':
      return []
    case 'help':
      return ['Mock shell. Try: ls, pwd, whoami, hostname, env, ps, uname']
    case 'ls':
      return ['app  bin  dev  etc  home  lib  proc  root  sys  tmp  usr  var']
    case 'pwd':
      return ['/']
    case 'whoami':
      return ['root']
    case 'hostname':
      return [podName]
    case 'uname':
      return [`Linux ${podName} 6.6.16 #1 SMP x86_64 GNU/Linux`]
    case 'env':
      return [
        `HOSTNAME=${podName}`,
        'PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin',
        'KUBERNETES_SERVICE_HOST=10.96.0.1',
        ...env.map((e) => `${e.name}=${e.value}`),
      ]
    case 'ps':
      return ['PID   USER     COMMAND', `1     root     /app/${container}`, '27    root     sh']
    case 'cat':
      return args.length ? [`cat: ${args[0]}: Permission denied (mock)`] : ['usage: cat FILE']
    default:
      return [`/bin/sh: ${bin}: command not found`]
  }
}

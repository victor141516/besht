import { spawnSync, type SpawnSyncOptions } from "node:child_process"
import { existsSync, statSync, accessSync, constants, appendFileSync, writeFileSync } from "node:fs"
import * as path from "node:path"

// ─── Command class ────────────────────────────────────────────────────────────

type StderrTarget = "null" | "&1" | string

interface PipelineStage {
  argv: string[]
}

class Command {
  private stages: PipelineStage[]
  private stdoutTarget: string | null = null
  private stdoutAppend: boolean = false
  private stderrTarget: StderrTarget | null = null
  private hasRun: boolean = false
  private capturedStdout: string = ""
  private capturedStderr: string = ""
  private capturedExit: number = 0
  _stdoutWasRead: boolean = false

  constructor(stages: PipelineStage[]) {
    this.stages = stages
  }

  pipe(cmd: Command): Command {
    const next = new Command([...this.stages, ...cmd.stages])
    next.stdoutTarget = this.stdoutTarget
    next.stdoutAppend = this.stdoutAppend
    next.stderrTarget = this.stderrTarget
    return next
  }

  stdout(filePath: string, mode?: string): Command {
    const next = this.clone()
    next.stdoutTarget = filePath === "null" ? "/dev/null" : filePath
    next.stdoutAppend = mode === "append"
    return next
  }

  stderr(target: StderrTarget): Command {
    const next = this.clone()
    next.stderrTarget = target
    return next
  }

  run(): this {
    if (this.hasRun) {
      throw new Error("Command already run. Use .clone() to run it again.")
    }
    this.hasRun = true

    let input: Buffer | undefined = undefined

    for (let i = 0; i < this.stages.length; i++) {
      const stage = this.stages[i]
      const isLast = i === this.stages.length - 1

      const opts: SpawnSyncOptions = {
        input,
        encoding: "buffer",
        maxBuffer: 100 * 1024 * 1024,
      }

      const result = spawnSync(stage.argv[0], stage.argv.slice(1), opts)

      if (isLast) {
        const rawOut = result.stdout as unknown as Buffer ?? Buffer.alloc(0)
        const rawErr = result.stderr as unknown as Buffer ?? Buffer.alloc(0)
        this.capturedExit = result.status ?? 0
        this.capturedStderr = rawErr.toString("utf8")

        if (this.stderrTarget === "&1") {
          this.capturedStdout = rawOut.toString("utf8") + this.capturedStderr
        } else if (this.stderrTarget === "null") {
          this.capturedStdout = rawOut.toString("utf8")
        } else if (this.stderrTarget && this.stderrTarget !== "null") {
          appendFileSync(this.stderrTarget, this.capturedStderr)
          this.capturedStdout = rawOut.toString("utf8")
        } else {
          process.stderr.write(rawErr)
          this.capturedStdout = rawOut.toString("utf8")
        }

        if (this.stdoutTarget) {
          const content = Buffer.isBuffer(rawOut) ? rawOut : Buffer.from(rawOut)
          if (this.stdoutAppend) {
            appendFileSync(this.stdoutTarget, content)
          } else {
            writeFileSync(this.stdoutTarget, content)
          }
        } else {
          const cmdRef = this
          const flush = () => {
            if (!cmdRef._stdoutWasRead && !cmdRef.stdoutTarget) {
              process.stdout.write(rawOut)
            }
          }
          setImmediate(flush)
        }
      } else {
        input = (result.stdout ?? Buffer.alloc(0)) as unknown as Buffer
      }
    }

    return this
  }

  readStdout(): string {
    this._stdoutWasRead = true
    return this.capturedStdout.replace(/\n$/, "")
  }

  readStdoutLines(): string[] {
    this._stdoutWasRead = true
    const t = this.capturedStdout.replace(/\n$/, "")
    if (t === "") return []
    return t.split("\n")
  }

  readStderr(): string {
    return this.capturedStderr.replace(/\n$/, "")
  }

  exitCode(): number {
    return this.capturedExit
  }

  clone(): Command {
    const next = new Command([...this.stages])
    next.stdoutTarget = this.stdoutTarget
    next.stdoutAppend = this.stdoutAppend
    next.stderrTarget = this.stderrTarget
    return next
  }
}

// ─── $ function ───────────────────────────────────────────────────────────────

function $(cmd: string, ...args: string[]): Command {
  return new Command([{ argv: [cmd, ...args] }])
}

// ─── env ──────────────────────────────────────────────────────────────────────

function env(name: string, defaultValue?: string): string {
  return process.env[name] ?? defaultValue ?? ""
}

function fetch(url: string): { text(): string } {
  const result = spawnSync("curl", ["-sS", "--", url], {
    encoding: "utf8",
    maxBuffer: 100 * 1024 * 1024,
  })
  if (result.stderr) process.stderr.write(result.stderr)
  const body = (result.stdout ?? "").replace(/\n+$/, "")
  return { text: () => body }
}

const argOptionNames = new Set<string>()
const argFlagNames = new Set<string>()

const args = {
  configure(options: string[], flags: string[]): void {
    for (const option of options) argOptionNames.add(option)
    for (const flag of flags) argFlagNames.add(flag)
  },
  optionNames(): Set<string> {
    return argOptionNames
  },
  flagNames(): Set<string> {
    return argFlagNames
  },
  argv(): string[] {
    const out: string[] = []
    const raw = process.argv.slice(2)
    const options = this.optionNames()
    const flags = this.flagNames()
    for (let i = 0; i < raw.length; i++) {
      const item = raw[i]
      if (item === "--") {
        out.push(...raw.slice(i + 1))
        break
      }
      if (item.startsWith("--") && item.includes("=")) {
        const name = item.slice(0, item.indexOf("="))
        if (options.has(name) || flags.has(name)) continue
      }
      if (options.has(item)) {
        i++
        continue
      }
      if (flags.has(item)) continue
      out.push(item)
    }
    return out
  },
  positional(n: number): string | undefined {
    return this.argv()[n - 1]
  },
  option(longName: string, shortName?: string): string | undefined {
    const raw = process.argv.slice(2)
    for (let i = 0; i < raw.length; i++) {
      const item = raw[i]
      if (item === "--") return undefined
      if (item === `--${longName}`) return raw[i + 1]
      if (item.startsWith(`--${longName}=`)) return item.slice(longName.length + 3)
      if (shortName && item === `-${shortName}`) return raw[i + 1]
    }
    return undefined
  },
  flag(longName: string, shortName?: string): boolean {
    const raw = process.argv.slice(2)
    for (const item of raw) {
      if (item === "--") return false
      if (item === `--${longName}` || (!!shortName && item === `-${shortName}`)) return true
    }
    return false
  },
}

// ─── type conversion ──────────────────────────────────────────────────────────

function to_str(value: any): string {
  return String(value)
}

function to_int(value: string): number {
  const n = parseInt(value.trim(), 10)
  return isNaN(n) ? 0 : n
}

const RuntimeString = String
const RuntimeNumber = Number

// ─── exit ─────────────────────────────────────────────────────────────────────

function exit(code: number = 0): never {
  process.exit(code)
}

// ─── file tests ───────────────────────────────────────────────────────────────

function file_exists(p: string): boolean {
  try { return statSync(p).isFile() } catch { return false }
}

function is_dir(p: string): boolean {
  try { return statSync(p).isDirectory() } catch { return false }
}

function is_readable(p: string): boolean {
  try { accessSync(p, constants.R_OK); return true } catch { return false }
}

function is_writable(p: string): boolean {
  try { accessSync(p, constants.W_OK); return true } catch { return false }
}

function is_executable(p: string): boolean {
  try { accessSync(p, constants.X_OK); return true } catch { return false }
}

function is_empty(s: string): boolean {
  return s === ""
}

function is_set(s: string): boolean {
  return s !== ""
}

// ─── list / array builtins ────────────────────────────────────────────────────

function len(arr: any[]): number {
  return arr.length
}

function head(arr: any[]): any {
  return arr[0]
}

function tail(arr: any[]): any[] {
  return arr.slice(1)
}

function append(arr: any[], item: any): any[] {
  return [...arr, item]
}

function contains(arr: any[], item: any): boolean {
  return arr.includes(item)
}

function concat(a: any[], b: any[]): any[] {
  return [...a, ...b]
}

function range(start: number, end: number): number[] {
  const result: number[] = []
  for (let i = start; i <= end; i++) result.push(i)
  return result
}

// ─── console (already global, but standardise stderr) ─────────────────────────

const _consoleError = console.error.bind(console)
Object.assign(console, {
  error: (...args: any[]) => _consoleError(...args),
})

// ─── exports ──────────────────────────────────────────────────────────────────

export {
  Command,
  $,
  args,
  env,
  fetch,
  to_str,
  to_int,
  RuntimeString as String,
  RuntimeNumber as Number,
  exit,
  file_exists,
  is_dir,
  is_readable,
  is_writable,
  is_executable,
  is_empty,
  is_set,
  len,
  head,
  tail,
  append,
  contains,
  concat,
  range,
}

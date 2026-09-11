import { spawn } from "node:child_process";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const web = join(root, "web");
const binary = join(root, "bin", "calculator");
const environmentFile = join(root, ".env");
const children = new Map();
const interruption = new AbortController();
let interrupted = false;
let shutdownPromise;

function terminate(child, signal) {
  if (!child.pid) return;
  try {
    process.kill(-child.pid, signal);
  } catch (error) {
    if (error.code !== "ESRCH") throw error;
  }
}

function start(command, args, cwd = root) {
  if (shutdownPromise) throw new Error("Task interrupted.");
  const child = spawn(command, args, { cwd, stdio: "inherit", detached: true });
  const finished = new Promise((resolve, reject) => {
    child.once("error", (error) => reject(new Error(`${command}: ${error.message}`)));
    child.once("exit", (code, signal) => resolve({ code, signal }));
  }).finally(() => {
    children.delete(child);
    // Remove descendants even if their direct parent exits first.
    terminate(child, "SIGKILL");
  });
  children.set(child, finished);
  return finished;
}

function exitDescription({ code, signal }) {
  return signal ? `stopped by ${signal}` : `exited with code ${code}`;
}

async function run(command, args, cwd = root) {
  const result = await start(command, args, cwd);
  if (result.code !== 0) throw new Error(`${command} ${exitDescription(result)}.`);
}

function shutdown() {
  if (shutdownPromise) return shutdownPromise;
  const active = [...children.entries()];
  shutdownPromise = (async () => {
    const timeout = setTimeout(() => {
      for (const [child] of active) terminate(child, "SIGKILL");
    }, 7000);
    try {
      for (const [child] of active) terminate(child, "SIGTERM");
      await Promise.allSettled(active.map(([, finished]) => finished));
    } finally {
      clearTimeout(timeout);
    }
  })();
  return shutdownPromise;
}

for (const [signal, code] of [["SIGINT", 130], ["SIGTERM", 143]]) {
  process.on(signal, () => {
    if (interrupted) {
      for (const child of children.keys()) terminate(child, "SIGKILL");
      return;
    }
    interrupted = true;
    process.exitCode = code;
    interruption.abort();
    void shutdown();
  });
}

async function initializeEnvironment() {
  const content = await readFile(join(root, ".env.example"), "utf8");
  try {
    await writeFile(environmentFile, content, { flag: "wx", mode: 0o600 });
    console.log("Created .env from .env.example.");
  } catch (error) {
    if (error.code !== "EEXIST") throw error;
    console.log("Existing .env left unchanged.");
  }
}

function loadEnvironment() {
  try {
    process.loadEnvFile(environmentFile);
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }
}

async function buildGo() {
  await mkdir(join(root, "bin"), { recursive: true });
  await run("go", ["build", "-o", binary, "./cmd/calculator"]);
}


async function develop() {
  await buildGo();
  const servers = [
    { name: "Go server", finished: start(binary, []) },
    { name: "Vite", finished: start(process.execPath, [join(web, "node_modules", "vite", "bin", "vite.js")], web) },
  ];
  const { name, result } = await Promise.race(servers.map(async ({ name, finished }) => ({
    name,
    result: await finished,
  })));
  throw new Error(`${name} ${exitDescription(result)}; stopping development servers.`);
}

const tasks = {
  env: initializeEnvironment,
  async setup() {
    await initializeEnvironment();
    loadEnvironment();
    await run("go", ["mod", "download"]);
    await run("npm", ["ci"], web);
  },
  dev: develop,
  "build-go": buildGo,
  "build-web": () => run("npm", ["run", "build"], web),
  async build() {
    await buildGo();
    await tasks["build-web"]();
  },
  async check() {
    await run("go", ["build", "./..."]);
    await run("go", ["vet", "./..."]);
    await run("go", ["test", "./..."]);
    await tasks["build-web"]();
  },
};

try {
  const name = process.argv[2];
  if (!Object.hasOwn(tasks, name)) {
    throw new Error(`Choose a task: ${Object.keys(tasks).join(", ")}.`);
  }
  loadEnvironment();
  await tasks[name]();
} catch (error) {
  if (!interrupted) {
    console.error(error.message);
    process.exitCode = 1;
  }
} finally {
  await shutdown();
}

import { execFileSync } from "node:child_process";
import { mkdirSync, rmSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";

export const targets = {
  "linux-x64": ["linux", "amd64"], "linux-arm64": ["linux", "arm64"],
  "darwin-x64": ["darwin", "amd64"], "darwin-arm64": ["darwin", "arm64"],
  "win32-x64": ["windows", "amd64"], "win32-arm64": ["windows", "arm64"],
};
export const extensionRoot = fileURLToPath(new URL("../", import.meta.url));
export const hostTarget = `${process.platform}-${process.arch}`;

export function buildBackend(target = hostTarget) {
  const platform = targets[target];
  if (!platform) { throw new Error(`Unsupported target: ${target}. Choose ${Object.keys(targets).join(", ")}.`); }
  mkdirSync(path.join(extensionRoot, "bin"), { recursive: true });
  const executable = platform[0] === "windows" ? "ghist-backend.exe" : "ghist-backend";
  execFileSync("go", ["build", "-trimpath", "-ldflags=-s -w", "-o", path.join(extensionRoot, "bin", executable), "./cmd/ghist-backend"], {
    cwd: path.resolve(extensionRoot, "../.."), stdio: "inherit",
    env: { ...process.env, CGO_ENABLED: "0", GOOS: platform[0], GOARCH: platform[1] },
  });
  // Never ship a stale backend for a different operating system in a VSIX.
  rmSync(path.join(extensionRoot, "bin", executable === "ghist-backend" ? "ghist-backend.exe" : "ghist-backend"), { force: true });
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  buildBackend(process.argv[2]);
}

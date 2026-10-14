import { execFileSync } from "node:child_process";
import { mkdirSync, mkdtempSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { runTests } from "@vscode/test-electron";
import { extensionRoot } from "./build-backend.mjs";

const temporary = mkdtempSync(path.join(tmpdir(), "ghist-vscode-test-"));
const git = (...args) => execFileSync("git", args, {
  cwd: temporary, stdio: "pipe",
  env: { ...process.env, GIT_CONFIG_NOSYSTEM: "1", GIT_CONFIG_GLOBAL: path.join(temporary, "absent-config") },
});
try {
  git("init", "-b", "main");
  git("config", "user.name", "Extension Test");
  git("config", "user.email", "test@example.com");
  git("config", "commit.gpgsign", "false");
  writeFileSync(path.join(temporary, "old file.txt"), "first\nsecond\nthird\n");
  git("add", "-A"); git("commit", "-m", "create");
  git("mv", "old file.txt", "renamed [x] $file.txt"); git("commit", "-m", "rename");
  writeFileSync(path.join(temporary, "renamed [x] $file.txt"), "first\nchanged\nthird\n");
  git("add", "-A"); git("commit", "-m", "edit");
  git("remote", "add", "origin", "https://github.com/acme/ghist-test.git");
  const fakeBin = path.join(temporary, "test-bin");
  mkdirSync(fakeBin);
  const reviewTest = process.platform !== "win32";
  const userData = path.join(temporary, "user-data");
  if (reviewTest) {
    writeFileSync(path.join(fakeBin, "gh"), "#!/bin/sh\nprintf '%s\\n' '[{\"number\":42,\"html_url\":\"https://github.com/acme/ghist-test/pull/42\"}]'\n", { mode: 0o755 });
    const backendWrapper = path.join(temporary, "ghist-backend-test");
    writeFileSync(backendWrapper,
      `#!/bin/sh\nPATH='${fakeBin}':"$PATH"\nexport PATH\nexec '${path.join(extensionRoot, "bin", "ghist-backend")}'\n`,
      { mode: 0o755 });
    const settingsDir = path.join(userData, "User");
    mkdirSync(settingsDir, { recursive: true });
    writeFileSync(path.join(settingsDir, "settings.json"), JSON.stringify({ "ghist.backendPath": backendWrapper }));
  }
  await runTests({
    ...(process.env.VSCODE_EXECUTABLE_PATH ? { vscodeExecutablePath: process.env.VSCODE_EXECUTABLE_PATH } : {}),
    extensionDevelopmentPath: process.env.GHIST_TEST_EXTENSION_PATH || extensionRoot,
    extensionTestsPath: path.join(extensionRoot, "out/test/suite/index.js"),
    extensionTestsEnv: {
      GHIST_TEST_REPOSITORY: temporary,
      ...(reviewTest ? { GHIST_TEST_REVIEWS: "1" } : {}),
    },
    launchArgs: [temporary, "--disable-extensions", "--disable-workspace-trust", "--no-sandbox", "--disable-gpu",
      "--remote-debugging-port=18742", "--skip-welcome", "--skip-release-notes", "--user-data-dir", userData],
  });
} finally {
  rmSync(temporary, { recursive: true, force: true });
}

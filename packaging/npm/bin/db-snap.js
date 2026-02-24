#!/usr/bin/env node

const fs = require("node:fs");
const path = require("node:path");
const { spawn } = require("node:child_process");

const binaryPath = path.join(__dirname, "db-snap-bin");

if (!fs.existsSync(binaryPath)) {
  console.error("db-snap binary is missing. Reinstall with: npm install -g db-snap");
  process.exit(1);
}

const child = spawn(binaryPath, process.argv.slice(2), { stdio: "inherit" });

child.on("error", (err) => {
  console.error(`failed to run db-snap: ${err.message}`);
  process.exit(1);
});

child.on("exit", (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code ?? 1);
});

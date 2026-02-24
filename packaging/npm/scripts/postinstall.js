const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const https = require("node:https");
const crypto = require("node:crypto");
const { execFileSync } = require("node:child_process");

const pkg = require("../package.json");

const platformMap = {
  darwin: "darwin",
  linux: "linux",
};

const archMap = {
  x64: "amd64",
  arm64: "arm64",
};

function fail(message) {
  console.error(`db-snap install failed: ${message}`);
  process.exit(1);
}

function download(url, destination, redirects = 0) {
  return new Promise((resolve, reject) => {
    const req = https.get(
      url,
      {
        headers: {
          "User-Agent": "db-snap-npm-installer",
          Accept: "application/octet-stream,text/plain,*/*",
        },
      },
      (res) => {
        if ([301, 302, 307, 308].includes(res.statusCode) && res.headers.location) {
          if (redirects > 5) {
            reject(new Error("too many redirects"));
            return;
          }
          download(res.headers.location, destination, redirects + 1).then(resolve).catch(reject);
          return;
        }

        if (res.statusCode !== 200) {
          reject(new Error(`download failed (${res.statusCode})`));
          return;
        }

        const out = fs.createWriteStream(destination);
        res.pipe(out);
        out.on("finish", () => out.close(resolve));
        out.on("error", reject);
      }
    );

    req.on("error", reject);
  });
}

function sha256(filePath) {
  const hash = crypto.createHash("sha256");
  hash.update(fs.readFileSync(filePath));
  return hash.digest("hex");
}

function findFile(root, name) {
  const entries = fs.readdirSync(root, { withFileTypes: true });
  for (const entry of entries) {
    const full = path.join(root, entry.name);
    if (entry.isDirectory()) {
      const found = findFile(full, name);
      if (found) {
        return found;
      }
      continue;
    }
    if (entry.name === name) {
      return full;
    }
  }
  return null;
}

async function install() {
  const platform = platformMap[os.platform()];
  const arch = archMap[os.arch()];
  if (!platform || !arch) {
    fail(`unsupported platform/arch: ${os.platform()}/${os.arch()}`);
  }

  const repo = process.env.DB_SNAP_REPO || (pkg.config && pkg.config.repo);
  if (!repo || !repo.includes("/")) {
    fail("missing GitHub repo. Set package.json config.repo to owner/name.");
  }

  const version = pkg.version;
  const assetName = `db-snap_${version}_${platform}_${arch}.tar.gz`;
  const base = `https://github.com/${repo}/releases/download/v${version}`;
  const archiveURL = `${base}/${assetName}`;
  const checksumsURL = `${base}/checksums.txt`;

  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "db-snap-install-"));
  const archivePath = path.join(tempDir, assetName);
  const checksumsPath = path.join(tempDir, "checksums.txt");

  try {
    await download(archiveURL, archivePath);
    await download(checksumsURL, checksumsPath);

    const checksums = fs.readFileSync(checksumsPath, "utf8").split(/\r?\n/);
    const line = checksums.find((item) => item.trim().endsWith(` ${assetName}`) || item.trim().endsWith(`  ${assetName}`));
    if (!line) {
      fail(`checksum entry not found for ${assetName}`);
    }
    const expected = line.trim().split(/\s+/)[0];
    const actual = sha256(archivePath);
    if (expected !== actual) {
      fail("checksum mismatch for downloaded archive");
    }

    execFileSync("tar", ["-xzf", archivePath, "-C", tempDir], { stdio: "ignore" });
    const extractedBinary = findFile(tempDir, "db-snap");
    if (!extractedBinary) {
      fail("db-snap binary not found in release archive");
    }

    const targetPath = path.join(__dirname, "..", "bin", "db-snap-bin");
    fs.copyFileSync(extractedBinary, targetPath);
    fs.chmodSync(targetPath, 0o755);
    console.log(`db-snap ${version} installed (${platform}/${arch}).`);
  } finally {
    fs.rmSync(tempDir, { recursive: true, force: true });
  }
}

install().catch((err) => fail(err.message));

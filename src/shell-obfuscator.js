'use strict';

const fs = require('node:fs');
const bashObfuscate = require('bash-obfuscate');

const DEFAULT_CHUNK_SIZE = 4;
const DEFAULT_OUTPUT_FILE = '/tmp/mdm-cli-obfuscated.sh';

function obfuscateShell(source, options = {}) {
  if (typeof source !== 'string' || source.length === 0 || source.includes('\0')) {
    throw new TypeError('Shell source must be non-empty text');
  }

  const chunkSize = options.chunkSize || DEFAULT_CHUNK_SIZE;
  const randomize = options.randomize !== false;
  const body = bashObfuscate(source, chunkSize, randomize);
  return `#!/bin/bash\n${body}\n`;
}

function createShellObfuscator(filePath, options = {}) {
  const outputPath = options.outputPath || DEFAULT_OUTPUT_FILE;

  return function getObfuscatedShell() {
    try {
      return fs.readFileSync(outputPath);
    } catch (error) {
      if (error.code !== 'ENOENT') throw error;
    }

    const source = fs.readFileSync(filePath, 'utf8');
    const output = Buffer.from(obfuscateShell(source, options), 'utf8');
    const temporaryPath = `${outputPath}.${process.pid}.${Date.now()}.tmp`;

    try {
      fs.writeFileSync(temporaryPath, output, { flag: 'wx', mode: 0o600 });
      fs.renameSync(temporaryPath, outputPath);
    } finally {
      try {
        fs.unlinkSync(temporaryPath);
      } catch (error) {
        if (error.code !== 'ENOENT') throw error;
      }
    }

    return fs.readFileSync(outputPath);
  };
}

module.exports = {
  createShellObfuscator,
  obfuscateShell
};

import { createRequire } from "node:module";

const packages = new Map([
  ["darwin:x64", "@operatorstack/yield-darwin-amd64"],
  ["darwin:arm64", "@operatorstack/yield-darwin-arm64"],
  ["linux:x64", "@operatorstack/yield-linux-amd64"],
  ["linux:arm64", "@operatorstack/yield-linux-arm64"],
  ["win32:x64", "@operatorstack/yield-windows-amd64"],
  ["win32:arm64", "@operatorstack/yield-windows-arm64"],
]);

export function runtimePackage(platform = process.platform, arch = process.arch) {
  const name = packages.get(`${platform}:${arch}`);
  if (!name) {
    throw new Error(`Yield does not provide a runtime for ${platform}/${arch}`);
  }
  return name;
}

export function resolveRuntime({
  platform = process.platform,
  arch = process.arch,
  resolve = createRequire(import.meta.url).resolve,
} = {}) {
  const name = runtimePackage(platform, arch);
  try {
    return resolve(name);
  } catch (error) {
    throw new Error(
      `The runtime package ${name} is missing. Reinstall @operatorstack/yield for ${platform}/${arch}.`,
      { cause: error },
    );
  }
}

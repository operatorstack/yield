export const targets = [
  {
    id: "darwin-amd64",
    goos: "darwin",
    goarch: "amd64",
    nodeOs: "darwin",
    nodeCpu: "x64",
    pythonTag: "macosx_11_0_x86_64",
    rustOs: "macos",
    rustArch: "x86_64",
  },
  {
    id: "darwin-arm64",
    goos: "darwin",
    goarch: "arm64",
    nodeOs: "darwin",
    nodeCpu: "arm64",
    pythonTag: "macosx_11_0_arm64",
    rustOs: "macos",
    rustArch: "aarch64",
  },
  {
    id: "linux-amd64",
    goos: "linux",
    goarch: "amd64",
    nodeOs: "linux",
    nodeCpu: "x64",
    pythonTag: "manylinux_2_17_x86_64",
    rustOs: "linux",
    rustArch: "x86_64",
  },
  {
    id: "linux-arm64",
    goos: "linux",
    goarch: "arm64",
    nodeOs: "linux",
    nodeCpu: "arm64",
    pythonTag: "manylinux_2_17_aarch64",
    rustOs: "linux",
    rustArch: "aarch64",
  },
  {
    id: "windows-amd64",
    goos: "windows",
    goarch: "amd64",
    nodeOs: "win32",
    nodeCpu: "x64",
    pythonTag: "win_amd64",
    rustOs: "windows",
    rustArch: "x86_64",
  },
  {
    id: "windows-arm64",
    goos: "windows",
    goarch: "arm64",
    nodeOs: "win32",
    nodeCpu: "arm64",
    pythonTag: "win_arm64",
    rustOs: "windows",
    rustArch: "aarch64",
  },
]

export function binaryName(target) {
  return `yskill-${target.goos}-${target.goarch}${target.goos === "windows" ? ".exe" : ""}`
}

export function npmPackage(target) {
  return `@operatorstack/yield-${target.id}`
}

export function rustPackage(target) {
  return `yieldskill-runtime-${target.id}`
}

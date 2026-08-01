# Release packaging

`assemble.mjs` turns six immutable `yskill` binaries into the language packages users install. It fails if any target is missing and writes a SHA-256 manifest.

```sh
node packaging/assemble.mjs --version 0.1.9 --binaries dist/bin --output dist/packages
```

The public TypeScript, Python, and Rust packages carry exactly one matching runtime through platform-specific package artifacts. They do not download a runtime or search `PATH` when invoked.

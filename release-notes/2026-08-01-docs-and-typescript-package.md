# Yield documentation and runnable TypeScript package

- Add a programmer-first documentation path: clean install, first workflow,
  primitive guides, tutorials, examples, conversion, and runtime reference.
- Compile the TypeScript SDK before packing so Node loads JavaScript from
  `node_modules` instead of refusing runtime type stripping there.
- Smoke-test both the source and projected npm tarballs in clean temporary
  projects before publication.

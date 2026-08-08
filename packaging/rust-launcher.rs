use sha2::{Digest, Sha256};
use std::fs::{self, OpenOptions};
use std::io::Write;
use std::path::{Path, PathBuf};
use std::process::Command;

#[cfg(all(target_os = "macos", target_arch = "x86_64"))]
use yieldskill_runtime_darwin_amd64 as runtime;
#[cfg(all(target_os = "macos", target_arch = "aarch64"))]
use yieldskill_runtime_darwin_arm64 as runtime;
#[cfg(all(target_os = "linux", target_arch = "x86_64"))]
use yieldskill_runtime_linux_amd64 as runtime;
#[cfg(all(target_os = "linux", target_arch = "aarch64"))]
use yieldskill_runtime_linux_arm64 as runtime;
#[cfg(all(target_os = "windows", target_arch = "x86_64"))]
use yieldskill_runtime_windows_amd64 as runtime;
#[cfg(all(target_os = "windows", target_arch = "aarch64"))]
use yieldskill_runtime_windows_arm64 as runtime;

fn runtime_path() -> Result<PathBuf, String> {
    let root = std::env::var_os("YIELD_RUNTIME_CACHE")
        .map(PathBuf::from)
        .unwrap_or_else(|| std::env::temp_dir().join("yieldskill"));
    let name = if cfg!(windows) {
        "yskill.exe"
    } else {
        "yskill"
    };
    let directory = root
        .join(env!("CARGO_PKG_VERSION"))
        .join(std::env::consts::ARCH);
    let path = directory.join(name);
    if verified(&path) {
        return Ok(path);
    }
    fs::create_dir_all(&directory)
        .map_err(|error| format!("could not create runtime cache: {error}"))?;
    let temporary = directory.join(format!(".{name}.{}.tmp", std::process::id()));
    let mut file = OpenOptions::new()
        .write(true)
        .create_new(true)
        .open(&temporary)
        .map_err(|error| format!("could not stage packaged runtime: {error}"))?;
    file.write_all(runtime::BYTES)
        .and_then(|_| file.sync_all())
        .map_err(|error| format!("could not write packaged runtime: {error}"))?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        fs::set_permissions(&temporary, fs::Permissions::from_mode(0o755))
            .map_err(|error| format!("could not mark packaged runtime executable: {error}"))?;
    }
    if let Err(error) = fs::rename(&temporary, &path) {
        let _ = fs::remove_file(&temporary);
        if !verified(&path) {
            return Err(format!("could not install packaged runtime: {error}"));
        }
    }
    if !verified(&path) {
        return Err("packaged runtime checksum mismatch".to_string());
    }
    Ok(path)
}

fn verified(path: &Path) -> bool {
    fs::read(path)
        .map(|bytes| hex::encode(Sha256::digest(bytes)) == runtime::SHA256)
        .unwrap_or(false)
}

fn main() {
    let path = match runtime_path() {
        Ok(path) => path,
        Err(error) => {
            eprintln!("yskill: {error}");
            std::process::exit(1);
        }
    };
    let args: Vec<_> = std::env::args_os().skip(1).collect();
    let mut command = Command::new(path);
    command.args(args);
    if std::env::var_os("YIELD_LANGUAGE").is_none() {
        command.env("YIELD_LANGUAGE", "rust");
    }
    if let Ok(launcher) = std::env::current_exe() {
        command.env("YIELD_LAUNCHER_PATH", launcher);
    }
    #[cfg(unix)]
    {
        use std::os::unix::process::CommandExt;
        let error = command.exec();
        eprintln!("yskill: could not start packaged runtime: {error}");
        std::process::exit(1)
    }
    #[cfg(windows)]
    {
        match command.status() {
            Ok(status) => std::process::exit(status.code().unwrap_or(1)),
            Err(error) => {
                eprintln!("yskill: could not start packaged runtime: {error}");
                std::process::exit(1);
            }
        }
    }
}

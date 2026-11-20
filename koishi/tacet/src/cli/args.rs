use clap::{Parser, Subcommand};
use std::path::PathBuf;

#[derive(Parser, Debug)]
#[command(
    name = "tacet",
    about = "Fast, zero-knowledge secret manager and environment injector"
)]
pub struct Cli {
    /// Custom Unix Domain Socket path
    #[arg(short, long, global = true)]
    pub socket: Option<PathBuf>,

    #[command(subcommand)]
    pub command: Commands,
}

#[derive(Subcommand, Debug)]
pub enum Commands {
    /// Interactive login, initial sync & daemon start
    Login {
        /// Bitwarden / Vaultwarden server URL (default: https://vault.bitwarden.com)
        #[arg(short, long, env = "TACET_SERVER")]
        server: Option<String>,

        /// Account email address
        #[arg(short, long, env = "TACET_EMAIL")]
        email: Option<String>,

        /// Authenticate using Bitwarden API Key (client_id and client_secret)
        #[arg(long)]
        apikey: bool,

        /// API Client ID (e.g. user.xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)
        #[arg(long, env = "TACET_CLIENT_ID")]
        client_id: Option<String>,

        /// API Client Secret
        #[arg(long, env = "TACET_CLIENT_SECRET")]
        client_secret: Option<String>,

        /// Master password for unlock (useful for non-interactive / CI environments)
        #[arg(long, env = "TACET_PASSWORD")]
        password: Option<String>,
    },

    /// Prompt Master Password to unlock tacetd
    Unlock {
        /// Master password (useful for non-interactive / script unlock)
        #[arg(long, env = "TACET_PASSWORD")]
        password: Option<String>,
    },

    /// Wipe keys in tacetd memory immediately
    Lock,

    /// Lock daemon & remove local tokens/cache
    Logout,

    /// Show login state, server URL, lock state, item count
    Status,

    /// Fetch latest encrypted vault from server
    Sync,

    /// List vault items and URI prefixes
    List {
        /// Optional scope filter (e.g. personal, personal/Dev, or org/<org>/<collection>)
        scope: Option<String>,

        /// Filter items by name or ID substring
        #[arg(short = 'q', long = "search")]
        search: Option<String>,

        /// Output raw JSON instead of tabular view
        #[arg(long)]
        json: bool,
    },

    /// Print single resolved secret to stdout
    Read {
        /// Target secret URI (e.g. tacet://personal/github/password)
        uri: String,

        /// Do not print trailing newline
        #[arg(short = 'n', long = "no-newline")]
        no_newline: bool,
    },

    /// Render {{ tacet://... }} placeholders (supports stdin/stdout)
    Inject {
        /// Template input file (reads from stdin if omitted)
        #[arg(short = 'i', long = "in")]
        input: Option<PathBuf>,

        /// Destination output file (writes to stdout if omitted)
        #[arg(short = 'o', long = "out")]
        output: Option<PathBuf>,
    },

    /// Inject resolved env vars into subprocess & mask output
    Run {
        /// Optional .env file to load placeholders from
        #[arg(long = "env-file")]
        env_file: Option<PathBuf>,

        /// Disable secret masking in stdout/stderr
        #[arg(long = "no-mask")]
        no_mask: bool,

        /// Command and arguments to execute
        #[arg(last = true, required = true)]
        command: Vec<String>,
    },
}

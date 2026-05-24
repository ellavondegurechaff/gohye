param(
    [string]$EnvFile = ".env.migration",
    [string]$BackupRoot = "mongo-backups",
    [string]$ReportsDir = "migration-reports",
    [switch]$SkipDump,
    [switch]$DumpOnly,
    [switch]$DirectMongo,
    [switch]$ResetBefore,
    [switch]$UseCopy
)

$ErrorActionPreference = "Stop"

function Read-DotEnv {
    param([string]$Path)

    if (!(Test-Path $Path)) {
        throw "Missing $Path. Copy .env.migration.example to $Path and fill it in."
    }

    Get-Content $Path | ForEach-Object {
        $line = $_.Trim()
        if ($line -eq "" -or $line.StartsWith("#")) {
            return
        }

        $parts = $line -split "=", 2
        if ($parts.Count -ne 2) {
            return
        }

        $name = $parts[0].Trim()
        $value = $parts[1].Trim().Trim('"').Trim("'")
        [Environment]::SetEnvironmentVariable($name, $value, "Process")
    }
}

function Require-Command {
    param([string]$Name)

    if (!(Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Required command '$Name' was not found in PATH."
    }
}

function Require-Env {
    param([string]$Name)

    $value = [Environment]::GetEnvironmentVariable($Name, "Process")
    if ([string]::IsNullOrWhiteSpace($value)) {
        throw "Missing required environment variable $Name in $EnvFile."
    }
    return $value
}

function Env-OrDefault {
    param([string]$Name, [string]$Default)

    $value = [Environment]::GetEnvironmentVariable($Name, "Process")
    if ([string]::IsNullOrWhiteSpace($value)) {
        return $Default
    }
    return $value
}

function Resolve-MongoDump {
    $cmd = Get-Command "mongodump" -ErrorAction SilentlyContinue
    if ($cmd) {
        return $cmd.Source
    }

    $knownPaths = @(
        "C:\Program Files\MongoDB\Tools\100\bin\mongodump.exe",
        "C:\Program Files\MongoDB\Database Tools\bin\mongodump.exe"
    )
    foreach ($path in $knownPaths) {
        if (Test-Path $path) {
            return $path
        }
    }

    return $null
}

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $repoRoot

Read-DotEnv -Path $EnvFile
Require-Command "go"

$mongoURI = Require-Env "MONGO_URI"
$mongoDB = Require-Env "MONGO_DB"
$pgHost = Env-OrDefault "PG_HOST" "localhost"
$pgPort = Env-OrDefault "PG_PORT" "5432"
$pgUser = Env-OrDefault "PG_USER" "root"
$pgPassword = Env-OrDefault "PG_PASSWORD" "root"
$pgDatabase = Env-OrDefault "PG_DATABASE" "postgres"
$batchSize = Env-OrDefault "MIGRATION_BATCH_SIZE" "1000"
$insertMode = Env-OrDefault "MIGRATION_INSERT_MODE" "batch"
$sleepMS = Env-OrDefault "MIGRATION_SLEEP_MS" "0"
$resetBeforeEnv = Env-OrDefault "MIGRATION_RESET_BEFORE" "false"
$resetOnError = Env-OrDefault "MIGRATION_RESET_ON_ERROR" "false"
$autoCreateMissing = Env-OrDefault "MIGRATION_AUTO_CREATE_MISSING_CARDS" "false"
$useCopyEnv = Env-OrDefault "MIGRATION_USE_COPY" "false"

if ($ResetBefore) {
    $resetBeforeEnv = "true"
}
if ($UseCopy) {
    $useCopyEnv = "true"
}

$timestamp = Get-Date -Format "yyyyMMdd_HHmmss"
New-Item -ItemType Directory -Force -Path $BackupRoot | Out-Null
New-Item -ItemType Directory -Force -Path $ReportsDir | Out-Null

$dumpRoot = Join-Path $BackupRoot $timestamp
$dataDir = Join-Path $dumpRoot $mongoDB
$mongoDumpCmd = Resolve-MongoDump

if (!$SkipDump -and !$DirectMongo) {
    if (!$mongoDumpCmd) {
        Write-Warning "mongodump was not found in PATH. Falling back to direct MongoDB migration without a local dump."
        $DirectMongo = $true
    }
}

if (!$SkipDump -and !$DirectMongo) {

    Write-Host "Creating MongoDB dump in $dumpRoot ..."
    & $mongoDumpCmd --uri $mongoURI --out $dumpRoot
    if ($LASTEXITCODE -ne 0) {
        throw "mongodump failed with exit code $LASTEXITCODE."
    }

    if (!(Test-Path $dataDir)) {
        $children = Get-ChildItem $dumpRoot -Directory
        if ($children.Count -eq 1) {
            $dataDir = $children[0].FullName
        } else {
            throw "Could not find dumped database directory for '$mongoDB' under $dumpRoot."
        }
    }
} elseif ($SkipDump) {
    if (!(Test-Path $dataDir)) {
        throw "SkipDump was set, but expected data directory does not exist: $dataDir"
    }
}

if (!$DirectMongo) {
    Write-Host "Mongo dump data directory: $dataDir"
}

if ($DumpOnly) {
    if ($DirectMongo) {
        throw "DumpOnly cannot be used when DirectMongo mode is active."
    }
    Write-Host "DumpOnly set; stopping before PostgreSQL migration."
    exit 0
}

$migrateArgs = @(
    "run", ".\bottemplate\cmd\migrate",
    "--data", $dataDir,
    "--host", $pgHost,
    "--port", $pgPort,
    "--user", $pgUser,
    "--password", $pgPassword,
    "--database", $pgDatabase,
    "--logdir", $ReportsDir,
    "--batch-size", $batchSize,
    "--insert-mode", $insertMode,
    "--sleep-ms", $sleepMS
)

if ($DirectMongo) {
    $migrateArgs += "--mongo-uri"
    $migrateArgs += $mongoURI
    $migrateArgs += "--mongo-db"
    $migrateArgs += $mongoDB
}

if ($resetBeforeEnv -eq "true") {
    $migrateArgs += "--reset-before"
}
if ($resetOnError -eq "true") {
    $migrateArgs += "--reset-on-error"
}
if ($autoCreateMissing -eq "true") {
    $migrateArgs += "--auto-create-missing-cards"
}
if ($useCopyEnv -eq "true") {
    $migrateArgs += "--use-copy"
}

if ($DirectMongo) {
    Write-Host "Starting direct MongoDB -> local PostgreSQL migration ..."
} else {
    Write-Host "Starting BSON dump -> local PostgreSQL migration ..."
}
Write-Host "Target PostgreSQL: $pgUser@$pgHost`:$pgPort/$pgDatabase"
& go $migrateArgs
if ($LASTEXITCODE -ne 0) {
    throw "migration failed with exit code $LASTEXITCODE."
}

Write-Host "Migration completed successfully."

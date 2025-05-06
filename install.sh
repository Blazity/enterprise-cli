#!/bin/bash
set -e

# Parse command line arguments
LOCAL_INSTALL=false
DEBUG_MODE=false

for arg in "$@"; do
  case $arg in
    --local)
      LOCAL_INSTALL=true
      shift # Remove --local from processing
      ;;
    --debug)
      DEBUG_MODE=true
      # Enable shell debugging if in debug mode
      set -x
      shift # Remove --debug from processing
      ;;
    *)
      # Unknown option
      ;;
  esac
done

# Function for debug logging
debug_log() {
  if [ "$DEBUG_MODE" = true ]; then
    echo -e "\n[DEBUG] $*" >&2
  fi
}

# Detect architecture (ARM or Intel)
IS_ARM=false
if [ "$(uname -m)" = "arm64" ] || [ "$(uname -m)" = "aarch64" ]; then
  IS_ARM=true
fi

# Determine common binary locations based on architecture
if [ "$IS_ARM" = true ]; then
  COMMON_BIN_DIRS="/opt/homebrew/bin:/opt/local/bin:/usr/local/bin:$HOME/bin"
  DEFAULT_INSTALL_DIR="/opt/homebrew/bin"
else
  COMMON_BIN_DIRS="/usr/local/bin:/opt/local/bin:$HOME/bin"
  DEFAULT_INSTALL_DIR="/usr/local/bin"
fi

# Define build variables similar to the Makefile
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT_SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
# Format LDFLAGS properly for go build command
LDFLAGS=""

# Colors for output - Based on the gif aesthetic
CYAN='\033[0;36m'
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
GRAY='\033[0;90m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Check for Homebrew and required packages on macOS
if [ "$(uname -s)" = "Darwin" ]; then
  if ! command -v brew &> /dev/null; then
    printf " ${RED}✗ Error: Homebrew is required but not found.${NC}\n"
    printf " ${GRAY}Install it from https://brew.sh/${NC}\n\n"
    exit 1
  fi

  for pkg in gh go; do
    if ! brew ls --versions "$pkg" &> /dev/null; then
      printf " ${RED}✗ Error: %s is not installed.${NC}\n" "$pkg"
      printf " ${GRAY}Please run: brew install %s${NC}\n\n" "$pkg"
      exit 1
    fi
  done
fi

# Spinner frames - More aesthetic spinner
SPINNER_FRAMES=('⣾' '⣽' '⣻' '⢿' '⡿' '⣟' '⣯' '⣷')

# Function to truncate paths to fit terminal width
truncate_path() {
  local path="$1"
  local max_length=${2:-40}  # Default max length of 40 chars
  
  # Get the terminal width if available
  if command -v tput &> /dev/null; then
    local term_width=$(tput cols)
    # If terminal width is available, use 65% of it
    if [[ -n "$term_width" ]]; then
      max_length=$((term_width * 65 / 100))
    fi
  fi
  
  if [[ ${#path} -gt $max_length ]]; then
    # Smart truncation - keep beginning and end
    local path_length=${#path}
    local chars_to_show=$((max_length - 3)) # 3 chars for ellipsis
    local start_chars=$((chars_to_show / 2))
    local end_chars=$((chars_to_show - start_chars))
    
    echo "${path:0:$start_chars}...${path:$((path_length-end_chars)):$end_chars}"
  else
    echo "$path"
  fi
}

# Function to display a spinner for a command
spinner() {
  local pid=$1
  local message=$2
  local i=0
  local delay=0.08
  
  while kill -0 $pid 2>/dev/null; do
    printf "\r${CYAN}%s${NC} ${GRAY}%s${NC}" " ${SPINNER_FRAMES[i]}" "$message"
    i=$(( (i+1) % ${#SPINNER_FRAMES[@]} ))
    sleep $delay
  done
  
  # Clear the spinner line
  printf "\r                                                               \r"
}

# Function to simulate a spinner for local testing
simulate_spinner() {
  local message=$1
  local duration=${2:-1}  # Default to 1 second for testing
  local i=0
  local delay=0.08
  
  # Calculate iterations based on duration and delay
  local iterations=$(echo "$duration / $delay" | bc)
  
  for ((j=0; j<iterations; j++)); do
    printf "\r${CYAN}%s${NC} ${GRAY}%s${NC}" " ${SPINNER_FRAMES[i]}" "$message"
    i=$(( (i+1) % ${#SPINNER_FRAMES[@]} ))
    sleep $delay
  done
  
  # Clear the spinner line
  printf "\r                                                               \r"
}

# Simple centered header
printf "\n"
printf " ${BOLD}${CYAN}Enterprise CLI Installer${NC}\n"
printf " ${GRAY}✦─────────────────────────✦${NC}\n"
if [ "$LOCAL_INSTALL" = true ]; then
  printf " ${YELLOW}⚙ Local development mode${NC}\n"
fi
if [ "$DEBUG_MODE" = true ]; then
  printf " ${YELLOW}🔍 Debug mode enabled${NC}\n"
fi
printf "\n"

if [ "$DEBUG_MODE" = true ]; then
  debug_log "OS: $(uname -a)"
  debug_log "Current working directory: $(pwd)"
  debug_log "User: $(whoami)"
  debug_log "PATH: $PATH"
  
  if [ -f "go.mod" ]; then
    debug_log "go.mod exists in current directory"
    debug_log "$(cat go.mod | head -5)"
  else
    debug_log "WARNING: go.mod not found in current directory"
  fi
fi

# Function to find Go binary in common locations
find_go_binary() {
  # Check in all common binary directories
  for dir in $(echo "$COMMON_BIN_DIRS" | tr ':' ' '); do
    if [ -x "$dir/go" ]; then
      echo "$dir/go"
      return 0
    fi
  done
  
  # Check Homebrew locations (specific to macOS)
  if [ -x "/opt/homebrew/bin/go" ]; then
    echo "/opt/homebrew/bin/go"
    return 0
  elif [ -x "/usr/local/bin/go" ]; then
    echo "/usr/local/bin/go"
    return 0
  fi
  
  # Check common Go installation locations
  if [ -x "/usr/local/go/bin/go" ]; then
    echo "/usr/local/go/bin/go"
    return 0
  elif [ -x "$HOME/go/bin/go" ]; then
    echo "$HOME/go/bin/go"
    return 0
  fi
  
  # Not found
  return 1
}

# Check if Go is installed
GO_CMD=""
if command -v go &> /dev/null; then
  GO_CMD="go"
else
  # Try to find Go in common locations
  GO_CMD=$(find_go_binary)
  
  if [ -z "$GO_CMD" ]; then
    printf " ${RED}✗ Error: Go is not installed${NC}\n"
    printf " ${GRAY}Go could not be found in PATH or common installation directories.${NC}\n"
    printf " ${GRAY}Please install Go from https://golang.org/dl/${NC}\n\n"
    exit 1
  else
    printf " ${YELLOW}⚠ Go not found in PATH but located at:${NC} ${CYAN}$GO_CMD${NC}\n"
    printf " ${GRAY}Will use this Go binary for installation.${NC}\n"
  fi
fi

# Check Go version
GO_VERSION=$($GO_CMD version | awk '{print $3}' | sed 's/go//')
REQUIRED_VERSION="1.24"

printf " ${GRAY}Go version:${NC} ${CYAN}$GO_VERSION${NC} "
if [[ "$(printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$REQUIRED_VERSION" ]]; then
    printf "${RED}✗${NC}\n"
    printf " ${RED}Error: Go version $REQUIRED_VERSION or higher is required${NC}\n"
    printf " ${GRAY}Please upgrade Go from https://golang.org/dl/${NC}\n\n"
    exit 1
else
    printf "${GREEN}✓${NC}\n"
fi

# Show architecture info if ARM
if [ "$IS_ARM" = true ]; then
  printf " ${GRAY}Architecture:${NC} ${CYAN}ARM64${NC}\n"
  printf " ${GRAY}Using installation directories for Apple Silicon${NC}\n"
fi

# Create a temporary directory
TMP_DIR=$(mktemp -d)
TRUNCATED_TMP_DIR=$(truncate_path "$TMP_DIR")

printf " ${GRAY}Preparing installation...${NC}\n"

if [ "$LOCAL_INSTALL" = true ]; then
  # In local mode, copy the current directory to the temp dir
  # First, check if we're in the repository directory
  if [ ! -f "go.mod" ] || [ ! -f "main.go" ]; then
    printf " ${RED}✗ Local installation requires running from the repository root${NC}\n"
    printf " ${GRAY}Please run this script from the enterprise-cli repository directory${NC}\n"
    
    if [ "$DEBUG_MODE" = true ]; then
      debug_log "Local install check failed:"
      debug_log "go.mod exists: $([ -f "go.mod" ] && echo "Yes" || echo "No")"
      debug_log "main.go exists: $([ -f "main.go" ] && echo "Yes" || echo "No")"
      debug_log "Current directory contents:"
      ls -la | head -10 >> "$BUILD_LOG"
    fi
    
    rm -rf "$TMP_DIR"
    exit 1
  fi
  
  if [ "$DEBUG_MODE" = true ]; then
    debug_log "Local installation mode - using current directory as source"
    debug_log "Current directory: $(pwd)"
    debug_log "Target temp directory: $TMP_DIR/enterprise-cli"
    
    # Copy with verbose output in debug mode
    mkdir -p "$TMP_DIR/enterprise-cli"
    debug_log "Copying local repository files"
    
    # Create the build log file if it doesn't exist yet
    if [ -z "$BUILD_LOG" ]; then
      BUILD_LOG=$(mktemp)
      debug_log "Created build log at: $BUILD_LOG"
    fi
    
    # Use rsync to copy files - handle the piping more carefully
    # Make sure we create the rsync log file first
    RSYNC_LOG="$BUILD_LOG.rsync"
    touch "$RSYNC_LOG" 
    
    rsync -av --exclude='.git' --exclude='node_modules' --exclude='vendor' --exclude='bin' . "$TMP_DIR/enterprise-cli/" > "$RSYNC_LOG" 2>&1
    RSYNC_EXIT=$?
    
    # Copy first few lines to the build log
    debug_log "rsync output (first few lines):"
    if [ -f "$RSYNC_LOG" ]; then
      head -20 "$RSYNC_LOG" >> "$BUILD_LOG" 2>/dev/null || true
    else
      debug_log "rsync log file wasn't created properly"
    fi
    
    debug_log "rsync exit code: $RSYNC_EXIT"
    debug_log "Files copied to temp directory:"
    ls -la "$TMP_DIR/enterprise-cli/" | head -10 >> "$BUILD_LOG"
    
    if [ $RSYNC_EXIT -ne 0 ]; then
      printf " ${RED}✗ Failed to copy local repository${NC}\n"
      debug_log "rsync failed with exit code $RSYNC_EXIT"
      exit 1
    fi
    
    printf " ${GREEN}✓${NC} ${GRAY}Using local repository${NC}\n"
  else
    # Simulate a spinner for better UI
    (sleep 2) &
    PID=$!
    spinner $PID "Using local repository"
    
    # Copy all files except .git directory to temp dir
    mkdir -p "$TMP_DIR/enterprise-cli"
    rsync -a --exclude='.git' --exclude='node_modules' --exclude='vendor' --exclude='bin' . "$TMP_DIR/enterprise-cli/" > /dev/null 2>&1
    
    if [ $? -ne 0 ]; then
      printf " ${RED}✗ Failed to copy local repository${NC}\n"
      rm -rf "$TMP_DIR"
      exit 1
    fi
    
    printf " ${GREEN}✓${NC} ${GRAY}Using local repository${NC}\n"
  fi
else
  # Regular mode - clone from GitHub
  exit_code_file=$(mktemp)
  (
    git clone https://github.com/blazity/enterprise-cli.git "$TMP_DIR/enterprise-cli" > /dev/null 2>&1
    echo $? > "$exit_code_file"
  ) &
  PID=$!
  spinner $PID "Cloning repository"
  
  # Get the exit code from the temp file
  exit_code=$(cat "$exit_code_file" 2>/dev/null || echo "")
  rm -f "$exit_code_file"
  
  # Default to failure if exit_code is not numeric
  if ! [[ "$exit_code" =~ ^[0-9]+$ ]]; then
    exit_code=1
  fi
  
  # Check if clone was successful
  if [ "$exit_code" -ne 0 ] || [ ! -d "$TMP_DIR/enterprise-cli" ]; then
    printf " ${RED}✗ Failed to clone repository${NC}\n"
    # Provide specific error messages based on git clone exit codes
    case "$exit_code" in
      128)
        printf " ${GRAY}Error: Unable to access repository. Check your network, repository URL, or authentication.${NC}\n"
        ;;
      129)
        printf " ${GRAY}Error: Invalid clone parameters. Please check command syntax or script version.${NC}\n"
        ;;
      130)
        printf " ${GRAY}Error: Clone process was interrupted by the user.${NC}\n"
        ;;
      *)
        printf " ${GRAY}Error: Git clone exited with code $exit_code.${NC}\n"
        ;;
    esac
    rm -rf "$TMP_DIR"
    exit 1
  fi
  
  printf " ${GREEN}✓${NC} ${GRAY}Repository cloned${NC}\n"
fi

# Navigate to the repository
cd "$TMP_DIR/enterprise-cli"

# Create a build log file if in debug mode
BUILD_LOG=""
if [ "$DEBUG_MODE" = true ]; then
  BUILD_LOG=$(mktemp)
  debug_log "Created build log at: $BUILD_LOG"
  debug_log "Go command: $GO_CMD"
  debug_log "LDFLAGS: $LDFLAGS"
  debug_log "Working directory: $(pwd)"
  
  # Verify the go.mod content and module structure
  debug_log "Checking Go module structure:"
  if [ -f "go.mod" ]; then
    debug_log "go.mod contents:"
    cat go.mod >> "$BUILD_LOG"
    
    # Check main.go and cmd directory
    if [ -f "main.go" ]; then
      debug_log "main.go exists"
    else
      debug_log "WARNING: main.go not found"
    fi
    
    if [ -d "cmd" ]; then
      debug_log "cmd directory contents:"
      ls -la cmd >> "$BUILD_LOG"
    else
      debug_log "WARNING: cmd directory not found"
    fi
  else
    debug_log "WARNING: go.mod not found in working directory"
  fi
fi

# Install dependencies
if [ "$DEBUG_MODE" = true ]; then
  debug_log "Running go mod tidy:"
  $GO_CMD mod tidy -v 2>&1 | tee -a "$BUILD_LOG"
  TIDY_EXIT_CODE=${PIPESTATUS[0]}
  debug_log "go mod tidy exit code: $TIDY_EXIT_CODE"
  
  if [ $TIDY_EXIT_CODE -ne 0 ]; then
    debug_log "go mod tidy failed"
    printf " ${RED}✗ Failed to install dependencies${NC}\n"
    if [ -f "$BUILD_LOG" ]; then
      printf "\n${YELLOW}Dependency Error Details:${NC}\n"
      tail -20 "$BUILD_LOG"
    fi
    exit 1
  fi
  printf " ${GREEN}✓${NC} ${GRAY}Dependencies installed${NC}\n"
else
  exit_code_file=$(mktemp)
  (
    $GO_CMD mod tidy > /dev/null 2>&1
    echo $? > "$exit_code_file"
  ) &
  PID=$!
  spinner $PID "Installing dependencies"
  
  exit_code=$(cat "$exit_code_file" 2>/dev/null || echo "1")
  rm -f "$exit_code_file"
  
  if ! [[ "$exit_code" =~ ^[0-9]+$ ]]; then
    exit_code=1
  fi
  
  if [ $exit_code -ne 0 ]; then
    printf " ${RED}✗ Failed to install dependencies${NC}\n"
    cd - > /dev/null
    rm -rf "$TMP_DIR"
    exit 1
  fi
  printf " ${GREEN}✓${NC} ${GRAY}Dependencies installed${NC}\n"
fi

# If we don't have a build log yet, create one
if [ "$DEBUG_MODE" = true ] && [ -z "$BUILD_LOG" ]; then
  BUILD_LOG=$(mktemp)
  debug_log "Created build log at: $BUILD_LOG"
  debug_log "Go command: $GO_CMD"
  debug_log "LDFLAGS: $LDFLAGS"
  debug_log "Current directory: $(pwd)"
  debug_log "Go module info:"
  $GO_CMD mod graph 2>&1 | head -5 >> "$BUILD_LOG" 2>/dev/null || true 
  debug_log "Go environment:"
  $GO_CMD env >> "$BUILD_LOG" 2>/dev/null || true
fi

# Build with debug output or spinner
exit_code_file=$(mktemp)
touch "$exit_code_file"
debug_log "Exit code file: $exit_code_file"
chmod 644 "$exit_code_file" # Make sure it's readable and writable

if [ "$DEBUG_MODE" = true ]; then
  # In debug mode, don't use spinner and show output
  debug_log "Building directly (no spinner)"
  
  # First build locally in bin directory
  mkdir -p bin
  debug_log "Created bin directory: $(pwd)/bin"
  debug_log "Files in current directory:"
  ls -la | head -20 >> "$BUILD_LOG"
  
  # Capture build output for debugging
  echo "=== Build Command Output ===" >> "$BUILD_LOG"
  
  # When in local mode, use special flags to ensure modules work
  if [ "$LOCAL_INSTALL" = true ]; then
    debug_log "Setting GO111MODULE=on for local build"
    # Use verbose flag and explicitly use module mode for debugging
    export GO111MODULE=on
    export GOFLAGS="-v -mod=mod"
    
    debug_log "Running build with environment: GO111MODULE=$GO111MODULE GOFLAGS=$GOFLAGS"
    
    # Run a preliminary check to see if modules are working
    $GO_CMD list -m all >> "$BUILD_LOG" 2>&1 || debug_log "Warning: go list modules failed"
    
    # For debugging, print the resolve path of the main module
    $GO_CMD list -f "{{.Dir}}" >> "$BUILD_LOG" 2>&1 || debug_log "Warning: go list module dir failed"
    
    # Run the build with full verbosity
    eval "$GO_CMD build -v $LDFLAGS -o bin/enterprise ." 2>&1 | tee -a "$BUILD_LOG"
  else
    eval "$GO_CMD build -v $LDFLAGS -o bin/enterprise ." 2>&1 | tee -a "$BUILD_LOG"
  fi
  BUILD_EXIT_CODE=${PIPESTATUS[0]}
  debug_log "Build exit code: $BUILD_EXIT_CODE"
  
  if [ $BUILD_EXIT_CODE -eq 0 ]; then
    debug_log "Build succeeded, installing binary"
    
    # 1. Try to install to GOPATH/bin
    echo "=== Go Install Output ===" >> "$BUILD_LOG"
    eval "$GO_CMD install $LDFLAGS" 2>&1 | tee -a "$BUILD_LOG"
    
    # 2. Copy to architecture-specific paths
    if [ "$IS_ARM" = true ]; then
      # For ARM, prefer /opt/homebrew/bin if it exists
      if [ -d "/opt/homebrew/bin" ] && [ -w "/opt/homebrew/bin" ]; then
        debug_log "Copying to ARM-specific path: /opt/homebrew/bin"
        cp bin/enterprise "/opt/homebrew/bin/" 2>&1 | tee -a "$BUILD_LOG"
        if [ $? -eq 0 ]; then
          debug_log "Successfully copied to /opt/homebrew/bin"
        else
          debug_log "Failed to copy to /opt/homebrew/bin"
        fi
      else
        debug_log "/opt/homebrew/bin not found or not writable"
      fi
    fi
    
    # 3. Try common locations in order of preference
    debug_log "Trying common installation directories: $COMMON_BIN_DIRS"
    for install_dir in $(echo "$COMMON_BIN_DIRS" | tr ':' ' '); do
      if [ -d "$install_dir" ] && [ -w "$install_dir" ]; then
        debug_log "Copying to: $install_dir"
        cp bin/enterprise "$install_dir/" 2>&1 | tee -a "$BUILD_LOG"
        if [ $? -eq 0 ]; then
          # Successfully copied to a directory in PATH
          echo "$install_dir" > "$exit_code_file.path"
          debug_log "Successfully copied to $install_dir"
          break
        else
          debug_log "Failed to copy to $install_dir"
        fi
      else
        debug_log "$install_dir not found or not writable"
      fi
    done
  else
    debug_log "Build failed, see log for details"
  fi
  
  echo $BUILD_EXIT_CODE > "$exit_code_file"
else
  # Regular non-debug mode with spinner
  (
    # First build locally in bin directory
    mkdir -p bin
    
    # Execute the build and capture errors
    if [ "$LOCAL_INSTALL" = true ]; then
      # Use GO111MODULE=on for local builds to avoid dependency issues
      export GO111MODULE=on
      export GOFLAGS="-mod=mod"
      BUILD_OUTPUT=$(eval "$GO_CMD build $LDFLAGS -o bin/enterprise ." 2>&1)
    else
      BUILD_OUTPUT=$(eval "$GO_CMD build $LDFLAGS -o bin/enterprise ." 2>&1)
    fi
    BUILD_EXIT_CODE=$?
    
    # If build succeeded, install it using multiple methods
    if [ $BUILD_EXIT_CODE -eq 0 ]; then
      # 1. Try to install to GOPATH/bin
      eval "$GO_CMD install $LDFLAGS" > /dev/null 2>&1
      
      # 2. Copy to architecture-specific paths
      if [ "$IS_ARM" = true ]; then
        # For ARM, prefer /opt/homebrew/bin if it exists
        if [ -d "/opt/homebrew/bin" ] && [ -w "/opt/homebrew/bin" ]; then
          cp bin/enterprise "/opt/homebrew/bin/" > /dev/null 2>&1
        fi
      fi
      
      # 3. Try common locations in order of preference
      for install_dir in $(echo "$COMMON_BIN_DIRS" | tr ':' ' '); do
        if [ -d "$install_dir" ] && [ -w "$install_dir" ]; then
          cp bin/enterprise "$install_dir/" > /dev/null 2>&1
          if [ $? -eq 0 ]; then
            # Successfully copied to a directory in PATH
            echo "$install_dir" > "$exit_code_file.path"
            break
          fi
        fi
      done
    else
      # Save the build error output
      echo "$BUILD_OUTPUT" > "$exit_code_file.error"
    fi
    
    # Return the build exit code
    echo $BUILD_EXIT_CODE > "$exit_code_file"
  ) &
  PID=$!
  spinner $PID "Building and installing"
fi

# Set BUILD_EXIT_CODE from the file in non-debug mode
if [ "$DEBUG_MODE" = false ]; then
  # Get the exit code from the temp file
  BUILD_EXIT_CODE=$(cat "$exit_code_file" 2>/dev/null || echo "1")
  
  # Check if build errors were captured
  if [ -f "$exit_code_file.error" ]; then
    BUILD_ERROR=$(cat "$exit_code_file.error")
    debug_log "Build error output captured: $BUILD_ERROR"
  fi
  
  # Clean up temp files unless in debug mode
  rm -f "$exit_code_file"
else
  debug_log "Using directly obtained BUILD_EXIT_CODE: $BUILD_EXIT_CODE"
fi

# Ensure BUILD_EXIT_CODE is a number
if ! [[ "$BUILD_EXIT_CODE" =~ ^[0-9]+$ ]]; then
  debug_log "Got invalid exit code, defaulting to 1"
  BUILD_EXIT_CODE=1
fi

# Check if build was successful
if [ $BUILD_EXIT_CODE -ne 0 ]; then
  printf " ${RED}✗ Failed to build${NC}\n"
  
  # Show error details in debug mode
  if [ "$DEBUG_MODE" = true ]; then
    # Display the last 20 lines of the build log directly 
    if [ -n "$BUILD_LOG" ] && [ -f "$BUILD_LOG" ]; then
      printf "\n${YELLOW}Build Error Output (last 20 lines):${NC}\n"
      tail -20 "$BUILD_LOG" | sed 's/^/  /'
      printf "\n${YELLOW}Full build log available at:${NC} ${CYAN}$BUILD_LOG${NC}\n"
      printf "You can view it with: ${CYAN}cat $BUILD_LOG${NC}\n"
    elif [ -n "$BUILD_ERROR" ]; then
      printf "\n${YELLOW}Build Error Details:${NC}\n"
      printf "%s\n" "$BUILD_ERROR" | sed 's/^/  /'
    fi
    
    # For local installation, show common issues
    if [ "$LOCAL_INSTALL" = true ]; then
      printf "\n${YELLOW}Common issues with local installation:${NC}\n"
      printf "  1. Module path issues - check go.mod's module name matches directory structure\n"
      printf "  2. Missing dependencies - run: ${CYAN}GO111MODULE=on go mod download -x${NC}\n"
      printf "  3. Go module caching issues - run: ${CYAN}go clean -modcache${NC}\n"
    fi
    
    # Show helpful go debug commands
    printf "\n${YELLOW}Troubleshooting commands:${NC}\n"
    printf "  ${CYAN}GO111MODULE=on $GO_CMD mod verify${NC}\n"
    printf "  ${CYAN}GO111MODULE=on $GO_CMD mod tidy -v${NC}\n"
    printf "  ${CYAN}GO111MODULE=on $GO_CMD build -v .${NC}\n"
    printf "  ${CYAN}GO111MODULE=on $GO_CMD list -m all${NC}\n\n"
  fi
  
  cd - > /dev/null
  
  # Don't remove TMP_DIR in debug mode to allow inspection
  if [ "$DEBUG_MODE" = false ]; then
    rm -rf "$TMP_DIR"
  else
    debug_log "Temp directory preserved for debugging: $TMP_DIR"
    printf "${YELLOW}Debug workspace preserved at:${NC} ${CYAN}$TMP_DIR${NC}\n"
  fi
  
  exit 1
fi

# Additional debug info about the built binary
if [ "$DEBUG_MODE" = true ] && [ -f "bin/enterprise" ]; then
  debug_log "Binary details:"
  file bin/enterprise >> "$BUILD_LOG"
  debug_log "Binary size: $(ls -lh bin/enterprise | awk '{print $5}')"
fi

# Verify the binary exists in the path or expected locations
if command -v enterprise &> /dev/null; then
  printf " ${GREEN}✓${NC} ${GRAY}Build and install successful${NC}\n"
else
  # Binary not in PATH, so create a message explaining how to add it
  GOPATH=${GOPATH:-$HOME/go}
  printf " ${GREEN}✓${NC} ${GRAY}Build successful${NC}\n"
  printf " ${YELLOW}⚠${NC} ${GRAY}Binary may not be in your PATH. You may need to:${NC}\n"
  printf "   ${GRAY}- Add ${CYAN}$GOPATH/bin${NC} ${GRAY}to your PATH${NC}\n"
  printf "   ${GRAY}- Run ${CYAN}source $SHELL_RC${NC} ${GRAY}to refresh your shell${NC}\n"
fi

# Detect shell type by checking the parent process command, fallback to $SHELL
detect_shell() {
    local parent_pid shell_cmd
    parent_pid=$(ps -o ppid= -p $$ 2>/dev/null | tr -d ' ')
    shell_cmd=$(ps -o comm= -p "$parent_pid" 2>/dev/null | xargs)
    case "$shell_cmd" in
        bash|zsh|fish|sh)
            echo "$shell_cmd";;
        *)
            if [ -n "$SHELL" ]; then
                echo "${SHELL##*/}"
            else
                echo "bash"
            fi;;
    esac
}

SHELL_TYPE=$(detect_shell)
printf " ${GRAY}Detected shell:${NC} ${CYAN}$SHELL_TYPE${NC}\n"

ALIAS_EXISTS=0
SHELL_RC=""

# Handle shell-specific configuration
if [ "$SHELL_TYPE" = "fish" ]; then
    # Fish shell uses a different config path and syntax
    SHELL_RC="$HOME/.config/fish/config.fish"
    
    # Create config directory if it doesn't exist
    mkdir -p "$HOME/.config/fish"
    
    # Check if alias already exists
    if grep -q "alias en='enterprise'" "$SHELL_RC" 2>/dev/null || \
       grep -q "abbr -a en enterprise" "$SHELL_RC" 2>/dev/null || \
       grep -q "alias en enterprise" "$SHELL_RC" 2>/dev/null; then
        ALIAS_EXISTS=1
    fi
    
    if [ "$ALIAS_EXISTS" -eq 0 ]; then
        # Use fish syntax for aliases
        echo 'alias en enterprise' >> "$SHELL_RC"
    fi
elif [ "$SHELL_TYPE" = "zsh" ]; then
    SHELL_RC="$HOME/.zshrc"
    # Check if alias already exists
    ALIAS_EXISTS=$(grep -c "alias en=['|\"]enterprise['|\"]" "$SHELL_RC" 2>/dev/null || true)
elif [ "$SHELL_TYPE" = "bash" ]; then
    # Check for different bash config files
    if [ -f "$HOME/.bash_profile" ]; then
        SHELL_RC="$HOME/.bash_profile"
    else
        SHELL_RC="$HOME/.bashrc"
    fi
    # Check if alias already exists
    ALIAS_EXISTS=$(grep -c "alias en=['|\"]enterprise['|\"]" "$SHELL_RC" 2>/dev/null || true)
else
    # Default to bashrc for unknown shells
    SHELL_RC="$HOME/.bashrc"
    # Check if alias already exists
    ALIAS_EXISTS=$(grep -c "alias en=['|\"]enterprise['|\"]" "$SHELL_RC" 2>/dev/null || true)
fi

# Add alias if it doesn't exist
if [ "$ALIAS_EXISTS" -eq 0 ] && [ -n "$SHELL_RC" ]; then
    TRUNCATED_SHELL_RC=$(truncate_path "$SHELL_RC")
    
    # Add alias with shell-specific syntax
    if [ "$SHELL_TYPE" = "fish" ]; then
        # Already added above for fish
        true
    else
        echo 'alias en="enterprise"' >> "$SHELL_RC"
    fi
    
    printf " ${GREEN}✓${NC} ${GRAY}Added alias ${CYAN}'en'${NC} ${GRAY}to${NC} ${CYAN}$TRUNCATED_SHELL_RC${NC}\n"
fi

# Clean up temporary files
cleanup_code_file=$(mktemp)
touch "$cleanup_code_file"
chmod 644 "$cleanup_code_file"

(
  cd - > /dev/null
  rm -rf "$TMP_DIR"
  echo $? > "$cleanup_code_file"
) &
PID=$!
spinner $PID "Finalizing installation"

# Get the exit code from the temp file
if [ -f "$cleanup_code_file" ]; then
  exit_code=$(cat "$cleanup_code_file" 2>/dev/null || echo "0")
  rm -f "$cleanup_code_file"
else
  # Default to success if file can't be read
  exit_code=0
fi

# Check if we built the binary successfully (independent of PATH)
BUILD_DIR="$TMP_DIR/enterprise-cli/bin"
if [ -f "$BUILD_DIR/enterprise" ]; then
    BUILT_SUCCESSFULLY=true
    LOCAL_BINARY="$BUILD_DIR/enterprise"
else
    BUILT_SUCCESSFULLY=false
fi

# Get installation directory from the previous step if available
INSTALL_DIR=""
if [ -f "$exit_code_file.path" ]; then
    INSTALL_DIR=$(cat "$exit_code_file.path" 2>/dev/null)
    rm -f "$exit_code_file.path"
fi

# Check if enterprise command is in PATH
if command -v enterprise &> /dev/null; then
    INSTALL_PATH=$(which enterprise 2>/dev/null)
    TRUNCATED_INSTALL_PATH=$(truncate_path "$INSTALL_PATH")
    
    printf "\n ${BOLD}${GREEN}✓ Installation successful!${NC}\n"
    printf " ${GRAY}Installed to:${NC} ${CYAN}$TRUNCATED_INSTALL_PATH${NC}\n\n"
    printf " ${GRAY}You can now use:${NC}\n"
    printf "   ${CYAN}enterprise${NC} ${GRAY}command${NC}    ${GRAY}# Run Enterprise CLI${NC}\n"
    printf "   ${CYAN}en${NC} ${GRAY}command${NC}            ${GRAY}# Using the alias (after sourcing)${NC}\n\n"
    printf " ${GRAY}Try running:${NC} ${CYAN}enterprise version${NC}\n"
    
    # If alias was added, show source command
    if [ "$ALIAS_EXISTS" -eq 0 ] && [ -n "$SHELL_RC" ]; then
        TRUNCATED_SHELL_RC=$(truncate_path "$SHELL_RC")
        printf " ${GRAY}To use the alias now, run:${NC} ${CYAN}source $TRUNCATED_SHELL_RC${NC}\n"
    fi
    
    printf " ${GRAY}✦─────────────────────────✦${NC}\n\n"
elif [ "$BUILT_SUCCESSFULLY" = true ]; then
    # Binary built but not in PATH
    GOPATH=${GOPATH:-$HOME/go}
    GOBIN="$GOPATH/bin"
    
    printf "\n ${BOLD}${YELLOW}⚠ Installation partially successful${NC}\n"
    printf " ${GRAY}Binary was built but might not be in your PATH.${NC}\n\n"
    
    # Check if we copied to an install directory
    if [ -n "$INSTALL_DIR" ] && [ -f "$INSTALL_DIR/enterprise" ]; then
        TRUNCATED_INSTALL_DIR=$(truncate_path "$INSTALL_DIR")
        printf " ${GRAY}Binary was installed to:${NC} ${CYAN}$TRUNCATED_INSTALL_DIR${NC}\n"
        printf " ${GRAY}You may need to add this directory to your PATH.${NC}\n\n"
    fi
    
    # If Go install succeeded, the binary might be in GOBIN
    if [ -f "$GOBIN/enterprise" ]; then
        printf " ${GRAY}Binary is available at:${NC} ${CYAN}$GOBIN/enterprise${NC}\n\n"
    elif [ -f "$LOCAL_BINARY" ]; then
        printf " ${GRAY}Binary is available at:${NC} ${CYAN}$LOCAL_BINARY${NC}\n\n"
    fi
    
    printf " ${GRAY}To complete installation, try one of these options:${NC}\n\n"
    
    # Provide customized instructions based on shell
    if [ "$SHELL_TYPE" = "fish" ]; then
        printf " ${GRAY}Option 1: Add Go bin to PATH:${NC}\n"
        printf "    ${CYAN}set -U fish_user_paths $GOBIN \$fish_user_paths${NC}\n\n"
        
        if [ -n "$INSTALL_DIR" ]; then
            printf " ${GRAY}Option 2: Add installation directory to PATH:${NC}\n"
            printf "    ${CYAN}set -U fish_user_paths $INSTALL_DIR \$fish_user_paths${NC}\n\n"
        fi
        
        printf " ${GRAY}Then reload your shell config:${NC}\n"
        printf "    ${CYAN}source ~/.config/fish/config.fish${NC}\n\n"
    else
        printf " ${GRAY}Option 1: Add Go bin to PATH in your shell config:${NC}\n"
        printf "    ${CYAN}export PATH=\$PATH:$GOBIN${NC}\n\n"
        
        if [ -n "$INSTALL_DIR" ]; then
            printf " ${GRAY}Option 2: Add installation directory to PATH:${NC}\n"
            printf "    ${CYAN}export PATH=\$PATH:$INSTALL_DIR${NC}\n\n"
        fi
        
        if [ -n "$SHELL_RC" ]; then
            TRUNCATED_SHELL_RC=$(truncate_path "$SHELL_RC")
            printf " ${GRAY}Then reload your shell config:${NC}\n"
            printf "    ${CYAN}source $TRUNCATED_SHELL_RC${NC}\n\n"
        fi
    fi
    
    # Provide manual run instructions
    if [ -f "$GOBIN/enterprise" ]; then
        printf " ${GRAY}Or you can run the binary directly:${NC}\n"
        printf "    ${CYAN}$GOBIN/enterprise${NC}\n"
    elif [ -f "$LOCAL_BINARY" ]; then
        printf " ${GRAY}Or you can copy the binary to a directory in your PATH:${NC}\n"
        printf "    ${CYAN}sudo cp $LOCAL_BINARY /usr/local/bin/${NC}\n"
    fi
    
    printf " ${GRAY}✦─────────────────────────✦${NC}\n\n"
else
    printf "\n ${BOLD}${RED}✗ Installation failed${NC}\n"
    if [ "$IS_ARM" = true ]; then
        printf " ${GRAY}The binary could not be built or installed correctly.${NC}\n"
        printf " ${GRAY}This might be due to architecture-specific issues with ARM.${NC}\n"
        printf " ${GRAY}Try running with GO_ENABLED=0 or GOARCH=arm64 flags.${NC}\n\n"
    else
        printf " ${GRAY}The binary could not be built or installed correctly.${NC}\n"
        printf " ${GRAY}Please check the Go configuration and try again.${NC}\n\n"
    fi
fi

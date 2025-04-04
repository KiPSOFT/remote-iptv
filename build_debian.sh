#!/bin/bash
set -e

echo "Starting Remote IPTV build process for Debian..."

# Define variables
APP_NAME="remote-iptv"
BUILD_DIR="./dist"
VERSION=$(date +"%Y%m%d%H%M")
INSTALL_DIR="/opt/${APP_NAME}"

# Make sure required tools are installed
echo "Checking for required build tools..."
command -v go >/dev/null 2>&1 || { echo "Go is required but not installed. Aborting."; exit 1; }
command -v npm >/dev/null 2>&1 || { echo "Node.js/npm is required but not installed. Aborting."; exit 1; }

# Create build directory
echo "Creating build directory..."
rm -rf ${BUILD_DIR}
mkdir -p ${BUILD_DIR}/${APP_NAME}
mkdir -p ${BUILD_DIR}/${APP_NAME}/bin
mkdir -p ${BUILD_DIR}/${APP_NAME}/web

# Build the React frontend
echo "Building React frontend..."
cd web
npm install
npm run build
cd ..
cp -r web/build/* ${BUILD_DIR}/${APP_NAME}/web/

# Build the Go backend
echo "Building Go backend..."
go mod tidy
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o ${BUILD_DIR}/${APP_NAME}/bin/${APP_NAME} ./cmd/server

# Create the database directory
mkdir -p ${BUILD_DIR}/${APP_NAME}/data
# Copy existing database if it exists
if [ -f "iptv.db" ]; then
    cp iptv.db ${BUILD_DIR}/${APP_NAME}/data/
fi

# Create a run script
cat > ${BUILD_DIR}/${APP_NAME}/run.sh << EOF
#!/bin/bash
# Make sure MPV is installed
command -v mpv >/dev/null 2>&1 || { echo "MPV is required but not installed. Please install with: apt install mpv"; exit 1; }

# Run the application
cd "\$(dirname "\$0")"
export PORT=8080
./bin/${APP_NAME}
EOF

# Make the run script executable
chmod +x ${BUILD_DIR}/${APP_NAME}/run.sh

# Create README file with instructions
cat > ${BUILD_DIR}/${APP_NAME}/README.txt << EOF
REMOTE IPTV APPLICATION
-----------------------

This is a web-based IPTV control system that allows you to manage IPTV channels
through a web interface without installing a desktop application.

REQUIREMENTS:
- MPV media player (install with: sudo apt install mpv)
- SQLite3 (usually pre-installed on Debian)

INSTALLATION:
1. Extract this archive to a location of your choice
   Example: sudo mkdir -p ${INSTALL_DIR} && sudo cp -r * ${INSTALL_DIR}

2. Make sure the application has the necessary permissions
   Example: sudo chown -R \$(whoami) ${INSTALL_DIR}

3. Install dependencies
   sudo apt update
   sudo apt install -y mpv sqlite3

USAGE:
1. Navigate to the installation directory: cd ${INSTALL_DIR}
2. Run the application: ./run.sh
3. Access the web interface: http://localhost:8080

The application will start in the terminal. Keep the terminal open to
keep the application running.
EOF

# Create a simple Debian install script
cat > ${BUILD_DIR}/install.sh << EOF
#!/bin/bash
set -e

# Check if running as root
if [ "\$(id -u)" != "0" ]; then
   echo "This script must be run as root" 1>&2
   exit 1
fi

# Install dependencies
apt update
apt install -y mpv sqlite3

# Create installation directory
mkdir -p ${INSTALL_DIR}

# Copy files
cp -r ${APP_NAME}/* ${INSTALL_DIR}/

# Set permissions
chmod +x ${INSTALL_DIR}/bin/${APP_NAME}
chmod +x ${INSTALL_DIR}/run.sh

echo ""
echo "Installation complete!"
echo "To run the application, go to ${INSTALL_DIR} and execute ./run.sh"
echo "Then access the web interface at http://localhost:8080"
EOF

chmod +x ${BUILD_DIR}/install.sh

# Create a tarball for distribution
echo "Creating distribution tarball..."
cd ${BUILD_DIR}
tar -czvf "${APP_NAME}-${VERSION}.tar.gz" ${APP_NAME} install.sh
cd ..

echo ""
echo "Build completed successfully!"
echo "The distribution package is available at: ${BUILD_DIR}/${APP_NAME}-${VERSION}.tar.gz"
echo ""
echo "To install on a Debian system:"
echo "1. Copy the tarball to the target system"
echo "2. Extract it: tar -xzvf ${APP_NAME}-${VERSION}.tar.gz"
echo "3. Run the installer: sudo ./install.sh"
echo ""
echo "After installation, run with: cd ${INSTALL_DIR} && ./run.sh" 
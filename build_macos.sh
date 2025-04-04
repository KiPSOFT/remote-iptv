#!/bin/bash
set -e

echo "Starting Remote IPTV build process for macOS..."

# Define variables
APP_NAME="RemoteIPTV"
BUILD_DIR="./dist"
VERSION=$(date +"%Y%m%d%H%M")
APP_BUNDLE="${BUILD_DIR}/${APP_NAME}.app"

# Make sure required tools are installed
echo "Checking for required build tools..."
command -v go >/dev/null 2>&1 || { echo "Go is required but not installed. Aborting."; exit 1; }
command -v npm >/dev/null 2>&1 || { echo "Node.js/npm is required but not installed. Aborting."; exit 1; }
command -v swiftc >/dev/null 2>&1 || { echo "Swift compiler is required but not installed. Aborting."; exit 1; }

# Create app bundle directory structure
echo "Creating app bundle directory structure..."
rm -rf ${APP_BUNDLE}
mkdir -p ${APP_BUNDLE}/Contents/MacOS
mkdir -p ${APP_BUNDLE}/Contents/Resources
mkdir -p ${APP_BUNDLE}/Contents/Resources/web
mkdir -p ${APP_BUNDLE}/Contents/Resources/data

# Build the React frontend
echo "Building React frontend..."
cd web
npm install
npm run build
cd ..
cp -r web/build/* ${APP_BUNDLE}/Contents/Resources/web/

# Build the Go backend
echo "Building Go backend..."
go mod tidy
CGO_ENABLED=1 GOOS=darwin go build -o ${APP_BUNDLE}/Contents/MacOS/remote-iptv ./cmd/server

# Create empty data directory but don't copy database
mkdir -p ${APP_BUNDLE}/Contents/Resources/data

# Create Swift app for menu bar icon
echo "Creating menu bar application wrapper..."
cat > menubar_app.swift << EOF
import Cocoa

class AppDelegate: NSObject, NSApplicationDelegate {
    let statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
    let serverProcess = Process()
    var serverURL = "http://localhost:8080"
    
    func applicationDidFinishLaunching(_ aNotification: Notification) {
        if let button = statusItem.button {
            button.image = NSImage(named: NSImage.Name("StatusBarButtonImage"))
            if button.image == nil {
                button.title = "📺"
            }
        }
        
        setupMenus()
        startServer()
        
        // Open browser after a short delay
        DispatchQueue.main.asyncAfter(deadline: .now() + 2.0) {
            NSWorkspace.shared.open(URL(string: self.serverURL)!)
        }
    }
    
    func setupMenus() {
        let menu = NSMenu()
        
        menu.addItem(NSMenuItem(title: "Open IPTV Interface", action: #selector(openInterface(_:)), keyEquivalent: "o"))
        menu.addItem(NSMenuItem.separator())
        menu.addItem(NSMenuItem(title: "Quit", action: #selector(quitApp(_:)), keyEquivalent: "q"))
        
        statusItem.menu = menu
    }
    
    func isMPVInstalled() -> Bool {
        // Check multiple possible locations for MPV
        let possiblePaths = [
            "/usr/local/bin/mpv",
            "/opt/homebrew/bin/mpv",
            "/usr/bin/mpv"
        ]
        
        // First check if any of the direct paths exist
        for path in possiblePaths {
            if FileManager.default.fileExists(atPath: path) {
                return true
            }
        }
        
        // Fall back to checking PATH with shell
        let task = Process()
        task.launchPath = "/bin/bash"
        task.arguments = ["-c", "command -v mpv"]
        
        let outputPipe = Pipe()
        task.standardOutput = outputPipe
        
        do {
            try task.run()
            task.waitUntilExit()
            
            let outputData = outputPipe.fileHandleForReading.readDataToEndOfFile()
            if let output = String(data: outputData, encoding: .utf8), !output.isEmpty {
                return true
            }
        } catch {
            print("Error checking for MPV: \(error)")
        }
        
        return false
    }
    
    func startServer() {
        // Check for MPV
        if !isMPVInstalled() {
            let alert = NSAlert()
            alert.messageText = "MPV Not Installed"
            alert.informativeText = "MPV is required but not installed. Please install with: brew install mpv"
            alert.alertStyle = .critical
            alert.addButton(withTitle: "OK")
            alert.runModal()
            NSApp.terminate(nil)
            return
        }
        
        // Start the server
        let appPath = Bundle.main.bundlePath
        let resourcesPath = "\(appPath)/Contents/Resources"
        let serverPath = "\(appPath)/Contents/MacOS/remote-iptv"
        
        serverProcess.launchPath = serverPath
        serverProcess.currentDirectoryPath = resourcesPath
        serverProcess.environment = [
            "PORT": "8080", 
            "PWD": resourcesPath,
            "WEB_ROOT": "\(resourcesPath)/web"  // Explicitly set web root path
        ]
        
        do {
            try serverProcess.run()
        } catch {
            let alert = NSAlert()
            alert.messageText = "Server Error"
            alert.informativeText = "Failed to start the IPTV server: \(error)"
            alert.alertStyle = .critical
            alert.addButton(withTitle: "OK")
            alert.runModal()
            NSApp.terminate(nil)
        }
    }
    
    @objc func openInterface(_ sender: Any?) {
        NSWorkspace.shared.open(URL(string: serverURL)!)
    }
    
    @objc func quitApp(_ sender: Any?) {
        serverProcess.terminate()
        NSApp.terminate(nil)
    }
    
    func applicationWillTerminate(_ aNotification: Notification) {
        serverProcess.terminate()
    }
}

// Main application entry point
let app = NSApplication.shared
let delegate = AppDelegate()
app.delegate = delegate
app.run()
EOF

# Create basic status bar icon (a simple TV icon)
mkdir -p ${APP_BUNDLE}/Contents/Resources
cat > icon_generator.swift << EOF
import Cocoa
import Foundation

let image = NSImage(size: NSSize(width: 18, height: 18))
image.lockFocus()

// Draw a simple TV icon
let tvRect = NSRect(x: 1, y: 3, width: 16, height: 12)
NSBezierPath.defaultLineWidth = 1.5
NSColor.white.set()
NSBezierPath(roundedRect: tvRect, xRadius: 2, yRadius: 2).stroke()

// TV screen
let screenRect = NSRect(x: 3, y: 5, width: 12, height: 8)
NSColor.white.set()
NSBezierPath(rect: screenRect).fill()

// TV stand
let standPath = NSBezierPath()
standPath.move(to: NSPoint(x: 7, y: 3))
standPath.line(to: NSPoint(x: 11, y: 3))
standPath.line(to: NSPoint(x: 11, y: 1))
standPath.line(to: NSPoint(x: 7, y: 1))
standPath.close()
NSColor.white.set()
standPath.fill()

image.unlockFocus()

// Save the image to the app bundle
if let tiffData = image.tiffRepresentation, 
   let bitmapImage = NSBitmapImageRep(data: tiffData), 
   let pngData = bitmapImage.representation(using: .png, properties: [:]) {
    try! pngData.write(to: URL(fileURLWithPath: "StatusBarButtonImage.png"))
}
EOF

# Compile and run the icon generator
echo "Generating status bar icon..."
swiftc icon_generator.swift -o icon_generator
./icon_generator
cp StatusBarButtonImage.png ${APP_BUNDLE}/Contents/Resources/

# Compile the Swift menu bar app
echo "Compiling menu bar application..."
swiftc menubar_app.swift -o ${APP_BUNDLE}/Contents/MacOS/launcher

# Make the launcher executable
chmod +x ${APP_BUNDLE}/Contents/MacOS/launcher

# Create Info.plist
cat > ${APP_BUNDLE}/Contents/Info.plist << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>launcher</string>
    <key>CFBundleIconFile</key>
    <string>AppIcon</string>
    <key>CFBundleIdentifier</key>
    <string>com.example.remoteiptv</string>
    <key>CFBundleInfoDictionaryVersion</key>
    <string>6.0</string>
    <key>CFBundleName</key>
    <string>${APP_NAME}</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0</string>
    <key>CFBundleVersion</key>
    <string>1</string>
    <key>LSMinimumSystemVersion</key>
    <string>10.13</string>
    <key>LSUIElement</key>
    <true/>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>NSHumanReadableCopyright</key>
    <string>Copyright © 2023. All rights reserved.</string>
    <key>NSPrincipalClass</key>
    <string>NSApplication</string>
</dict>
</plist>
EOF

# Create a simple icon or use a placeholder
mkdir -p ${APP_BUNDLE}/Contents/Resources/AppIcon.iconset
cat > ${APP_BUNDLE}/Contents/Resources/README.txt << EOF
Remote IPTV Application

This is a web-based IPTV control system that allows you to manage IPTV channels
through a web interface.

Requirements:
- MPV media player (install with: brew install mpv)

The application automatically opens a browser window to access the interface.
If the browser does not open automatically, click on the menu bar icon and select "Open IPTV Interface".

To quit the application, click on the menu bar icon and select "Quit".
EOF

# Clean up temporary files
rm -f menubar_app.swift icon_generator.swift icon_generator StatusBarButtonImage.png

# Create DMG for distribution (optional)
if command -v create-dmg >/dev/null 2>&1; then
    echo "Creating DMG package..."
    create-dmg \
        --volname "${APP_NAME}" \
        --volicon "AppIcon.icns" \
        --window-pos 200 120 \
        --window-size 600 400 \
        --icon-size 100 \
        --icon "${APP_NAME}.app" 175 120 \
        --hide-extension "${APP_NAME}.app" \
        --app-drop-link 425 120 \
        "${BUILD_DIR}/${APP_NAME}-${VERSION}.dmg" \
        "${APP_BUNDLE}" \
        || echo "Skipping DMG creation. To create DMGs, install create-dmg: brew install create-dmg"
else
    echo "Skipping DMG creation. To create DMGs, install create-dmg: brew install create-dmg"
    echo "Creating ZIP archive instead..."
    cd ${BUILD_DIR}
    zip -r "${APP_NAME}-${VERSION}.zip" "${APP_NAME}.app"
    cd ..
fi

echo ""
echo "Build completed successfully!"
echo "The application bundle is available at: ${APP_BUNDLE}"
if [ -f "${BUILD_DIR}/${APP_NAME}-${VERSION}.dmg" ]; then
    echo "DMG package available at: ${BUILD_DIR}/${APP_NAME}-${VERSION}.dmg"
elif [ -f "${BUILD_DIR}/${APP_NAME}-${VERSION}.zip" ]; then
    echo "ZIP archive available at: ${BUILD_DIR}/${APP_NAME}-${VERSION}.zip"
fi
echo ""
echo "To run the application, open the app bundle in Finder."
echo "Make sure MPV is installed: brew install mpv"
echo "The application will appear as an icon in your menu bar." 
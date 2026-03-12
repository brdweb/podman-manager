<?php
$docroot = $docroot ?? $_SERVER['DOCUMENT_ROOT'] ?: '/usr/local/emhttp';
require_once "$docroot/webGui/include/Wrappers.php";

header('Content-Type: application/json');

$action = $_POST['action'] ?? '';
$pluginDir = '/boot/config/plugins/podman-manager';
$configFile = "$pluginDir/config.yaml";
$keyFile = "$pluginDir/id_ed25519";
$binary = '/usr/local/bin/podman-manager';

switch ($action) {
    case 'backend_start':
        if (trim(shell_exec("pgrep -f $binary 2>/dev/null")) !== '') {
            echo json_encode(['success' => false, 'error' => 'Already running']);
            break;
        }
        exec("$binary --config $configFile > /var/log/podman-manager.log 2>&1 &");
        sleep(1);
        $running = trim(shell_exec("pgrep -f $binary 2>/dev/null")) !== '';
        echo json_encode(['success' => $running, 'error' => $running ? '' : 'Failed to start']);
        break;

    case 'backend_stop':
        exec("pkill -f $binary 2>/dev/null");
        sleep(1);
        echo json_encode(['success' => true]);
        break;

    case 'backend_restart':
        exec("pkill -f $binary 2>/dev/null");
        sleep(2);
        exec("$binary --config $configFile > /var/log/podman-manager.log 2>&1 &");
        sleep(1);
        $running = trim(shell_exec("pgrep -f $binary 2>/dev/null")) !== '';
        echo json_encode(['success' => $running]);
        break;

    case 'generate_key':
        if (file_exists($keyFile)) {
            echo json_encode(['success' => false, 'error' => 'Key already exists']);
            break;
        }
        exec("ssh-keygen -t ed25519 -f " . escapeshellarg($keyFile) . " -N '' 2>&1", $out, $ret);
        if ($ret === 0) {
            chmod($keyFile, 0600);
            $pubKey = file_get_contents("$keyFile.pub");
            echo json_encode([
                'success' => true,
                'message' => "SSH key generated.\n\nCopy this public key to your Podman hosts:\n\n$pubKey\n" .
                    "Run on each host:\n  ssh-copy-id -i $keyFile.pub your-user@<host-ip>"
            ]);
        } else {
            echo json_encode(['success' => false, 'error' => implode("\n", $out)]);
        }
        break;

    default:
        echo json_encode(['success' => false, 'error' => 'Unknown action']);
}

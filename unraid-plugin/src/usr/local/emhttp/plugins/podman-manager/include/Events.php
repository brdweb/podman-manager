<?php
$docroot = $docroot ?? $_SERVER['DOCUMENT_ROOT'] ?: '/usr/local/emhttp';
require_once "$docroot/webGui/include/Wrappers.php";

header('Content-Type: application/json');

$action = $_POST['action'] ?? $_GET['action'] ?? '';
$pluginDir = '/boot/config/plugins/podman-manager';
$configFile = "$pluginDir/config.yaml";
$keyFile = "$pluginDir/id_ed25519";
$binary = '/usr/local/bin/podman-manager';
$cfg = parse_plugin_cfg('podman-manager');
$apiPort = $cfg['API_PORT'] ?? '18734';

switch ($action) {
    case 'api_proxy':
        $path = $_GET['path'] ?? '';
        if (strpos($path, '/api/') !== 0) {
            http_response_code(400);
            echo json_encode(['error' => 'Invalid API path']);
            break;
        }
        $url = "http://127.0.0.1:$apiPort" . $path;
        $query = $_GET;
        unset($query['action'], $query['path']);
        if ($query) $url .= '?' . http_build_query($query);

        $opts = ['http' => ['timeout' => 10, 'ignore_errors' => true]];
        if ($_SERVER['REQUEST_METHOD'] === 'POST') {
            $opts['http']['method'] = 'POST';
            $opts['http']['header'] = 'Content-Type: application/json';
            $opts['http']['content'] = file_get_contents('php://input');
        }
        $ctx = stream_context_create($opts);
        $response = @file_get_contents($url, false, $ctx);
        if ($response === false) {
            http_response_code(502);
            echo json_encode(['error' => 'Backend unreachable']);
        } else {
            echo $response;
        }
        break;

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
                'message' => "SSH key generated.\n\nCopy this public key to each Podman host:\n\n$pubKey\n" .
                    "Note: the trailing host comment (for example, root@xwing) is just a label on the key and does not control which remote user is used.\n\n" .
                    "Replace your-user with the SSH account on the Podman host and replace <host-ip> with that host's address:\n" .
                    "  ssh-copy-id -i $keyFile.pub your-user@<host-ip>"
            ]);
        } else {
            echo json_encode(['success' => false, 'error' => implode("\n", $out)]);
        }
        break;

    default:
        echo json_encode(['success' => false, 'error' => 'Unknown action']);
}

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

function respond_json($payload, $status = 200) {
    http_response_code($status);
    echo json_encode($payload);
    exit;
}

function csrf_token_valid() {
    global $var;

    $expected = $var['csrf_token'] ?? '';
    $provided = $_POST['csrf_token'] ?? $_GET['csrf_token'] ?? ($_SERVER['HTTP_X_CSRF_TOKEN'] ?? '');

    return $expected !== '' && is_string($provided) && hash_equals($expected, $provided);
}

function require_csrf_token() {
    if (!csrf_token_valid()) {
        respond_json(['error' => 'Invalid CSRF token'], 403);
    }
}

function backend_status_code($headers) {
    if (!is_array($headers) || empty($headers[0])) {
        return 200;
    }

    if (preg_match('/\s(\d{3})\s/', $headers[0], $matches)) {
        return (int) $matches[1];
    }

    return 200;
}

function backend_running($binary) {
    return trim(shell_exec('pgrep -f ' . escapeshellarg($binary) . ' 2>/dev/null')) !== '';
}

function start_backend($binary, $configFile) {
    exec(escapeshellarg($binary) . ' --config ' . escapeshellarg($configFile) . ' > /var/log/podman-manager.log 2>&1 &');
}

function stop_backend($binary) {
    exec('pkill -f ' . escapeshellarg($binary) . ' 2>/dev/null');
}

switch ($action) {
    case 'api_proxy':
        $path = $_GET['path'] ?? '';
        if (strpos($path, '/api/') !== 0) {
            respond_json(['error' => 'Invalid API path'], 400);
        }

        $method = strtoupper($_SERVER['REQUEST_METHOD'] ?? 'GET');
        if ($method !== 'GET') {
            require_csrf_token();
        }

        $url = "http://127.0.0.1:$apiPort" . $path;
        $query = $_GET;
        unset($query['action'], $query['path']);
        unset($query['csrf_token']);
        if ($query) $url .= '?' . http_build_query($query);

        $opts = ['http' => [
            'method' => $method,
            'timeout' => 10,
            'ignore_errors' => true,
            'header' => 'Content-Type: application/json',
        ]];

        if (in_array($method, ['POST', 'PUT', 'PATCH', 'DELETE'], true)) {
            $opts['http']['content'] = file_get_contents('php://input');
        }

        $ctx = stream_context_create($opts);
        $response = @file_get_contents($url, false, $ctx);
        if ($response === false) {
            respond_json(['error' => 'Backend unreachable'], 502);
        } else {
            http_response_code(backend_status_code($http_response_header ?? []));
            echo $response;
        }
        break;

    case 'backend_start':
        require_csrf_token();
        if (backend_running($binary)) {
            respond_json(['success' => false, 'error' => 'Already running']);
        }
        start_backend($binary, $configFile);
        sleep(1);
        $running = backend_running($binary);
        echo json_encode(['success' => $running, 'error' => $running ? '' : 'Failed to start']);
        break;

    case 'backend_stop':
        require_csrf_token();
        stop_backend($binary);
        sleep(1);
        echo json_encode(['success' => true]);
        break;

    case 'backend_restart':
        require_csrf_token();
        stop_backend($binary);
        sleep(2);
        start_backend($binary, $configFile);
        sleep(1);
        $running = backend_running($binary);
        echo json_encode(['success' => $running]);
        break;

    case 'generate_key':
        require_csrf_token();
        if (file_exists($keyFile)) {
            respond_json(['success' => false, 'error' => 'Key already exists']);
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

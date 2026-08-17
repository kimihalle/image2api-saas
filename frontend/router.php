<?php

$uri = parse_url($_SERVER['REQUEST_URI'], PHP_URL_PATH) ?: '/';
$isProxy = str_starts_with($uri, '/admin/api/')
    || str_starts_with($uri, '/images/')
    || str_starts_with($uri, '/v1/')
    || $uri === '/health';

if ($isProxy) {
    $target = 'http://backend:6666' . $_SERVER['REQUEST_URI'];
    $headers = [];
    foreach (getallheaders() as $name => $value) {
        if (!in_array(strtolower($name), ['host', 'connection', 'content-length'], true)) {
            $headers[] = $name . ': ' . $value;
        }
    }
    $headers[] = 'X-Forwarded-Proto: http';
    $headers[] = 'X-Forwarded-Host: ' . ($_SERVER['HTTP_HOST'] ?? 'localhost');

    $context = stream_context_create(['http' => [
        'method' => $_SERVER['REQUEST_METHOD'],
        'header' => implode("\r\n", $headers),
        'content' => file_get_contents('php://input'),
        'ignore_errors' => true,
        'timeout' => 600,
    ]]);
    $body = @file_get_contents($target, false, $context);
    $responseHeaders = $http_response_header ?? [];
    foreach ($responseHeaders as $index => $header) {
        if ($index === 0 && preg_match('/\s(\d{3})\s/', $header, $match)) {
            http_response_code((int) $match[1]);
        } else {
            $name = strtolower(trim(strtok($header, ':')));
            if (in_array($name, ['content-type', 'content-disposition', 'set-cookie', 'location', 'etag', 'last-modified'], true)) {
                header($header, $name !== 'set-cookie');
            }
        }
    }
    header('Cache-Control: no-store');
    echo $body === false ? '' : $body;
    return true;
}

$path = realpath('/app' . $uri);
if ($uri !== '/' && $path !== false && str_starts_with($path, '/app/') && is_file($path)) {
    return false;
}

header('Cache-Control: no-cache, no-store, must-revalidate');
header('Content-Type: text/html; charset=utf-8');
readfile('/app/index.html');
return true;

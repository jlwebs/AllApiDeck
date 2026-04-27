$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
$session.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"

$headers = @{
    "authority"="ai.121628.xyz"
    "method"="GET"
    "path"="/api/status"
    "scheme"="https"
    "accept"="*/*"
    "accept-encoding"="gzip, deflate, br, zstd"
    "accept-language"="zh-CN,zh;q=0.9,en;q=0.8,ru;q=0.7"
    "cache-control"="no-cache"
    "pragma"="no-cache"
    "priority"="u=1, i"
    "sec-ch-ua"='`"Chromium`";v=`"146`", `"Not-A.Brand`";v=`"24`", `"Google Chrome`";v=`"146`"'
    "sec-ch-ua-mobile"="?0"
    "sec-ch-ua-platform"='`"Windows`"'
    "sec-fetch-dest"="empty"
    "sec-fetch-mode"="cors"
    "sec-fetch-site"="none"
}

try {
    Write-Host "Sending optimized request to ai.121628.xyz/api/status..." -ForegroundColor Cyan
    $response = Invoke-WebRequest -UseBasicParsing -Uri "https://ai.121628.xyz/api/status" `
        -WebSession $session `
        -Headers $headers `
        -ContentType "application/json"
    
    if ($response.StatusCode -eq 200) {
        Write-Host "Success! Status 200 Received." -ForegroundColor Green
        Write-Host "Response Body Preview:" -ForegroundColor Gray
        $response.Content | Select-Object -First 500
    } else {
        Write-Host "Failed! Status: $($response.StatusCode)" -ForegroundColor Red
    }
} catch {
    Write-Host "Error occurred:" -ForegroundColor Red
    $_.Exception.Message | Write-Host -ForegroundColor Yellow
}

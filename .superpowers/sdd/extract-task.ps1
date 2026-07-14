$plan = Get-Content 'd:\Projects\paytm-assignment\docs\superpowers\plans\2026-07-14-url-shortener.md' -Raw
$pattern = '(?ms)^### Task 1:.*?(?=\r?\n---\r?\n)'
$match = [regex]::Match($plan, $pattern)
if ($match.Success) {
    $match.Value | Out-File -FilePath 'd:\Projects\paytm-assignment\.superpowers\sdd\task-1-brief.md' -Encoding utf8 -NoNewline
    $lines = ($match.Value -split "`n").Count
    Write-Output "wrote task-1-brief.md: $lines lines"
} else {
    Write-Output 'Task 1 not found'
}

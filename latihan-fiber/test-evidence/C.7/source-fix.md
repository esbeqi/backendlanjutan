# C.7 source fix

Before: `if appErr.Status < fiber.StatusInternalServerError` sent 4xx to `request_failed` at Error level and sent 5xx to `request_rejected` at Warn level.

After: `if appErr.Status >= fiber.StatusInternalServerError` sends 5xx to `request_failed` at Error level and 4xx to `request_rejected` at Warn level.

Only the comparison direction changed in `config/app.go`. HTTP status/body behavior and the explanatory comment were retained. The duplicate-enrollment probe returned 201 then 409 before and after; the post-fix API log contains `request_rejected` and no `request_failed`.

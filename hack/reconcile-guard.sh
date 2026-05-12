#!/usr/bin/env bash
# Reconcile Guard — prevents bare r.Update() / r.Create() on managed resources.
# See docs/conventions.md "Reconciliation rules".

set -euo pipefail

fail=0

# Check for banned patterns in Go files
for file in $(find internal/controller -name "*.go" ! -name "*_test.go"); do
    # Read file and track which function we're in
    in_reconcile_pvc=0
    in_reconcile_cr=0
    line_num=0

    while IFS= read -r line; do
        line_num=$((line_num + 1))

        # Track function context
        if [[ $line =~ func\ \(.*\)\ reconcilePVC ]]; then
            in_reconcile_pvc=1
        elif [[ $line =~ func\ \(.*\)\ reconcileCR ]]; then
            in_reconcile_cr=1
        elif [[ $line =~ ^func\ \( ]]; then
            # Entering a different function
            in_reconcile_pvc=0
            in_reconcile_cr=0
        fi

        # Skip if line has allow comment
        if [[ $line =~ reconcile-guard:allow ]]; then
            continue
        fi

        # Skip if inside allowed functions
        if [[ $in_reconcile_pvc -eq 1 ]] || [[ $in_reconcile_cr -eq 1 ]]; then
            continue
        fi

        # Check for banned patterns
        if [[ $line =~ r\.Update\(ctx, ]]; then
            echo "::error::Banned pattern 'r.Update(ctx,' found at $file:$line_num:" >&2
            echo "$line" >&2
            fail=1
        fi

        if [[ $line =~ r\.Create\(ctx, ]]; then
            echo "::error::Banned pattern 'r.Create(ctx,' found at $file:$line_num:" >&2
            echo "$line" >&2
            fail=1
        fi
    done < "$file"
done

exit $fail

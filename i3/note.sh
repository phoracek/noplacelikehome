#!/usr/bin/env bash
#
# Based on https://demu.red/blog/2021/04/a-notational-velocity-stopgap-solution-for-linux/

function note() {
    pushd ~/Notes 1>/dev/null && \
    while true; do
        fuzzy_result=$(fzf -i --cycle --reverse --style=minimal --preview-window=down --preview='cat {}' --print-query)
        # Exit if Escape was hit
        if [ $? -eq 130 ]; then
            exit
        fi
        # Ignore empty file name
        if [ -z "$fuzzy_result" ]; then
            continue
        fi

        file=$(echo ${fuzzy_result} | gawk 'END{if($0 !~ /.md$/){$0=gensub(" ","_","g",$0) ".md"}; print $0}')

        hx $file
    done
    popd 1>/dev/null
}

note

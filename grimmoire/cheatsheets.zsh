#!/bin/zsh

CHEATSHEET_DIR="/Users/jdawson/Projects/darkness/cheatsheets/"
typeset -A files

for filepath in "$CHEATSHEET_DIR"/*-cheat.json; do
  filename=${filepath:t}                      # :t modifier = basename
  key=${filename%-cheat.json}                # remove suffix
  files[$key]="$filepath"
done

jless_flag='false'
run_flag='false'
verbose_flag='false'
edit_flag='false'
json_property=''
choice=''

# Parse arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        -j)
            jless_flag='true'
            shift
            if [[ "$1" && !("$1" =~ ^-) ]]; then
                json_property="$1"
                shift
            fi
            ;;
        -r)
            run_flag='true'
            shift
            ;;
        -e)
            edit_flag='true'
            shift
            ;;
        -v)
            verbose_flag='true'
            shift
            ;;
        *)
            if [[ -z "$choice" ]]; then
                choice="$1"
                shift
            else
                shift
            fi
            ;;
    esac
done

if [[ "$verbose_flag" == 'true' ]]; then
  echo "Arguments parsed: -j $jless_flag, -e $edit_flag, json_property: $json_property, Choice: $choice"
fi

if [[ -z $choice ]]; then
  if ! command -v fzf >/dev/null; then
    echo "fzf is not installed. Please install it or pass a cheatsheet name as an argument."
    exit 1
  fi
  choice=$(for k in "${(@k)files}"; do echo $k; done | fzf --prompt="Select a cheatsheet:")
fi

# Normalize to lowercase and check if it's a valid key
choice_lower=$(echo "$choice" | tr '[:upper:]' '[:lower:]')
if [[ -n "${files[$choice_lower]}" ]]; then
  choice=$choice_lower
fi

if [[ $edit_flag == 'true' ]]; then
  nvim "${files[$choice]}"
  exit
fi

# Get content
if [[ -n "${files[$choice]}" ]]; then
  content=$(cat "${files[$choice]}")

    if [[ "$verbose_flag" == 'true' ]]; then
      echo "Content prepared for display or processing..."
      echo $content
    fi

    if [[ -n "$json_property" ]]; then
        # Auto-expand shorthand like 'log' to 'log.commands' if using -r
        if [[ "$run_flag" == 'true' && "$json_property" != *.* ]]; then
            json_property=".$json_property.commands"
        elif [[ "$json_property" != .* ]]; then
            json_property=".$json_property"
        fi

        # If the value is an array of commands, let the user pick one
        is_array=$(echo "$content" | jq -r "$json_property | type" 2>/dev/null)
        if [[ "$is_array" == "array" ]]; then
            commands=$(echo "$content" | jq -r "$json_property[] | \"\(.command) # \(.description)\"" 2>/dev/null)
            selected=$(echo "$commands" | fzf --prompt="Select a command:")
            content="${selected%% #*}"
        else
            # Otherwise fetch the raw value
            content=$(echo "$content" | jq -r "$json_property" 2>/dev/null)
        fi
    fi

    if [[ "$run_flag" == 'true' ]]; then
      tmpfile=$(mktemp /tmp/cheat_run_XXXXXX.sh)
      echo "$content" > "$tmpfile"

      ${EDITOR:-nvim} "$tmpfile"

      read "run_confirm?Run the command? [y/N]: "
      if [[ "$run_confirm" != [Yy] ]]; then
        echo "Canceled."
        rm "$tmpfile"
        exit 0
      fi

      echo "Running edited command..."
      chmod +x "$tmpfile"
      "$tmpfile"
      rm "$tmpfile"
      exit
    fi

    if [[ "$jless_flag" == 'true' ]]; then
        echo "$content" | jless -m line
    else
        echo "$content"
    fi
else
  echo "Invalid selection. Please use one of the following: ${(@k)files}"
fi

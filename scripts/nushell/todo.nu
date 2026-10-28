# This is a simple todo list tool
# It uses a file (essentially a yaml list) to store quick notes
# The last command output is automatically store on the variable '$env.TODOS_LAST_RESULT'
# Allow listing, adding searching and completing tasks
# Use the 'help todo' or 'todo <command> -h' to learn more

export-env {$env.TODOS_LAST_RESULT = []}
 
def todo-file [
  --file (-f): string
] {
  ($file | default $env.TODO_MAIN_FILE)
}

def load-todos [--file: string] {

  let todo_path = (todo-file --file $file)
  (open $"($todo_path)" -r | from yaml)
}

def --env export-return [] : any -> any {
  let export_todo = $in
  export-env {$env.TODOS_LAST_RESULT = $export_todo}
  return $export_todo
}

def select-multiple [] : table -> table {
  let the_table = $in
  return ($the_table | (if (($in | length) > 1) {$in | insert tmp {|r| [$r.id ": " $r.summary] | str join }| input list --multi 'Select tasks wih space key, confim wiht enter key, abort with q or esc' -d tmp | try {reject -o tmp} } else {$in}));
}

def select-single [] : table -> table {
  let the_table = $in
  return ($the_table | (if (($in | length) > 1) {$in | insert tmp {|r| [$r.id ": " $r.summary] | str join }| input list 'Select tasks wih space key, confim wiht enter key, abort with q or esc' -d tmp | try {reject -o tmp} } else {$in}));
}

def full-fields [full: bool, hide_done: bool = false] : table -> table {
  let $the_table = $in
  return (if ($hide_done) {
    $the_table | if ($full) { $in | reject done } else {$in | select id summary date updated};
  } else {
    $the_table | update done { if ($in == true) { "✅"} else {"⏳"}} | if ($full) { $in } else {$in | select id done summary date updated};
  }) | update date { into datetime } | update updated { if ($in == null) {''} else { $in | into datetime} };
}

def tags-list [] {
  open $"($env.TODO_MAIN_FILE)" -r | from yaml | get tags | flatten | uniq;
}

# Helper to harmoize stdin and args
def collect-import-ids [
    in_args?: list<string>
    in_std?: list<any>
] : nothing -> list<string> {
    (($in_args | default []) | union ($in_std | default [] | into string))
}

# Generic upsert for all tasks
def upsert-task [
    task: record
    --file (-f): string
] {

    let todo_path = (todo-file --file $file)
    let todos = (load-todos --file $file)
    let link = ($task.links | first)

    let match = (
      $todos
        | where {|r|
            ($r.links | default [])
            | any {|l| $l == $link}
        })

    if (($match | length) == 0) {
        let new_id = (add --file $file --tags $task.tags --links $task.links $task.summary)
        return ["created" $new_id]
    }

    let existing_id = ($match | get 0.id)

    let updated = (
      $todos
        | each {|r|

            if $r.id == $existing_id {

                $r
                | upsert summary $task.summary
                | upsert tags $task.tags

            } else {
                $r
            }
        })

    $updated
    | to yaml
    | save $todo_path -f

    ["updated" $existing_id]
}

#######################
## Import helpers
#######################

##################
### Import ado
##################
 
def ado-pat [
    pat?: string
] {

    let found_pat = (
        $pat
        | default $env.TODO_ADO_PAT?
    )

    if ($found_pat | is-empty) {
        error make {
            msg: "No Azure DevOps PAT configured."
            help: "Run 'todo set pat ado' or pass '--pat'."
        }
    }

    $found_pat
}

# Build the azure devops api url
def ado-api-url [
    org: string
    project: string
    id: string
] {
    $"https://dev.azure.com/($org)/($project)/_apis/wit/workitems/($id)?api-version=7.1"
}

# Build the azure devops api url
def ado-workitem-url [
    org: string
    project: string
    id: string
] {
    $"https://dev.azure.com/($org)/($project)/_workitems/edit/($id)"
}

def ado-get-workitem [
    org: string
    project: string
    id: string
    pat?: string
] {
    let valid_pat = ado-pat $pat
    http get --password $valid_pat (ado-api-url $org $project $id)
}

def find-by-link [
    link: string
    --file (-f): string
] {

    let todos = (load-todos --file $file)

    $todos
    | where {|r|
        ($r.links | default [])
        | any {|l| $l == $link}
    }
}

##################
### Import github
##################

def github-pat [
  pat?: string
] {

  let found_pat = (
    $pat
    | default $env.TODO_GITHUB_PAT?
  )

  if ($found_pat | is-empty) {
    error make {
      msg: "No Github PAT configured."
            help: "Run 'todo set pat github' or pass '--pat'."
        }
    }

  $found_pat
}

def github-api-url [
  repo: string
  id: string
] {
  $"https://api.github.com/repos/($repo)/issues/($id)"
}

def github-issue-url [
    repo: string
    id: string
] {
    $"https://github.com/($repo)/issues/($id)"
}

def github-get-issue [
  repo: string
  id: string
  pat?: string
] {

  let headers = {
    Authorization: $"Bearer (github-pat $pat)"
        Accept: "application/vnd.github+json"
        X-GitHub-Api-Version: "2022-11-28"
    }

  http get --headers $headers (github-api-url $repo $id)
}


#######################
## Public fucntions
#######################

# Set a the base file on the env var TODO_MAIN_FILE
export def --env "set file" [
  file: string # path to the new file
] {
  let file = ($in | default $file)
  let expanded = ($file | path expand)
  let parent = ($expanded | path dirname)

  if not ($parent | path exists) {
    error make {
      msg: $"Directory does not exist: ($parent)"
        }
    }

  $env.TODO_MAIN_FILE = $expanded
}

# Set the pat on the env var TODO_ADO_PAT
export def --env "set pat github" [] {
  let stdin_val = $in
  $env.TODO_GITHUB_PAT = if ($stdin_val != null) {
    $stdin_val
  } else {
    (input --suppress-output "Github PAT")
  }
  print "PAT stored in current Nushell session."
}


# Upsert github issues from Github into the todo list
export def --env "import github" [
  --repo (-r): string
  --file (-f): string
  --pat: string
  ...ids: string
] {

  let ids = (collect-import-ids $ids $in)

  $ids
  | each {|id|

      try {

          let issue = (github-get-issue $repo $id $pat)

          let task = {

              summary: ($"($id) : ($issue.title)")

              tags: (
                  [
                      github
                      ($"repo:($repo)" | str replace '/' '_')
                  ]
                  ++ (
                      $issue.labels
                      | each {|l|
                          $l.name
                          | str replace ' ' '_'
                      }
                  )
              )

              links: [
                  $issue.html_url
              ]
          }

          let out = (upsert-task $task --file $file)

          {
              id: $out.1
              work_item: $id
              title: $task.summary
              action: $out.0
          }

      } catch {|e|

          {
              id: ""
              work_item: $id
              title: ($e.msg? | default ($e | to text))
              action: "error"
          }

      }
  }
}

# Set the pat on the env var TODO_ADO_PAT
export def --env "set pat ado" [] {
    let stdin_val = $in
    $env.TODO_ADO_PAT = if ($stdin_val != null) {
      $stdin_val
    } else {
     (input --suppress-output "Azure DevOps PAT")
    }
    print "PAT stored in current Nushell session."
}

# Upsert work items from Azure DevOps into the todo list
export def --env "import ado" [
    --org (-o): string     # The devops Organization
    --project (-p): string # The devops Project
    --file (-f): string    # Override the default todo.yml
    --pat: string          # Override the default devops PAT
    ...ids: string         # The list of ids to upsert
] {
    let ids = (collect-import-ids $ids $in)

    $ids
    | each {|id|
        try {
            let wi = ado-get-workitem $org $project $id $pat
            let link = ado-workitem-url $org $project $id
            let task = {

                summary: ($"($id) : ($wi.fields.'System.Title')")

                tags: [
                    ado
                    ($wi.fields.'System.AreaPath' | str replace ' ' '_')
                ]

                links: [
                    $link
                ]
            }

            let out = (upsert-task $task --file $file)

            {
                id: $out.1
                work_item: $id
                title: $task.summary
                action: $out.0
            }
        } catch {|e|
            {
                id: ""
                work_item: $id
                title: ($e.msg? | default ($e | to text))
                action: "error"
            }
        }
    }
}

# Import section

# List TODOs
export def --env list [
  --file (-f): path
  --all (-a) # Return all. By default show not done
  --full # Return all columns
] : nothing -> table {
  let todo_path = (todo-file --file $file)

  let the_list = open $"($todo_path)" -r | from yaml;
  if ($the_list == null) {
    return [];
  }
  return ((if $all {
    $the_list | full-fields $full;
  } else {
    $the_list |  where done == false | full-fields $full true;
  }) | export-return )
}

# Search a entries on the TODOs list
# If multiple matches are wound a list input picker is prompted
export def --env fuzzy [
  id?: string # the id or summary or a subset of it (might return multiples)
  --file (-f): path
] : [
    string -> table
    nothing -> table
   ] {
  let todo_path = (todo-file --file $file)

  if ($id == null and $in == null) {
    return;
  }

  let d_id = match ($in | describe) {
    'string' => $in,
    _ => $id
  }

  let the_list = open $"($todo_path)" -r | from yaml;

  return ($the_list | find $d_id | select-multiple | full-fields true | export-return);
}

# Get (search) an entry of the TODOs list
export def --env by-tag [
  tag?: string  # the tag or subset of it (might return multiples)
  --file (-f): path
  --select (-s) # prompt an input list to select from existing
] : [
    string -> table
    nothing -> table
   ] {
  let todo_path = (todo-file --file $file)
   
  if ($tag == null and $in == null and not $select) {
    return;
  }

  let d_tag = match ($in | describe) {
    'string' => $tag,
    _ => (if ($select) {tags-list | input list "Pick a tag"} else { $tag })
  }

  let the_list = open $"($todo_path)" -r | from yaml;

   return ($the_list | where {|row| ($row.tags | any { |t| $t =~ $d_tag })} | full-fields true | export-return );
  }

# Add an entry to the TODOs list
export def --env add [
  --done (-d), # mark as done
  --links (-l): list, # add links
  --tags (-t): list,  # add tags
  --tag-select (-s),  # add tags selected from a list
  --file (-f): string
  ...summary: string  # text to add
] : nothing -> string {
  print $file
  let todo_path = (todo-file --file $file)
  let text = ($summary | str join ' ');
  let new_date = (date now);
  let id = ([$text $new_date] | str join | hash md5 | str substring ..9);
  let concat_tags = [] ++ ($tags | default []) ++ (if $tag_select {tags-list | input list -m "Pick a tag"} else {[]})
  [{id: $id, done: (if ($done) {true} else {false}), summary: $text , date: $new_date , updated: (if ($done) {date now} else {''}), tags: $concat_tags, links: ($links | default [])}] | export-return | to yaml | save $"($todo_path)" -a
  return $id
}

# Complete a task (marking it as done)
#
# Uses the id to find the desired task
# If --fuzzy is set the id argument accepts a summary excerpt for fuzzy searching
# If multiple matches are wound a list input picker is prompted
export def --env done [
  id?: string   # The id or partial id to complete.
  --file (-f): path
] : [
    string -> table
    nothing -> table
   ] {
  let todo_path = (todo-file --file $file)

  if ($id == null and $in == null) {
    return [];
  }

  let d_id = match ($in | describe) {
    'string' => $in,
    _ => $id
  }

  let the_list = open $"($todo_path)" -r | from yaml;

  let found = ($the_list | where { |r| ($r.id like $d_id and $r.done == false)});
  let found_count = ($found | length)

  if ($found_count == 0) {
    return ( [] | export-return )
  }

  let selected_id = (if ($found_count == 1) {
    ($found | get 0.id)
  } else {
    ($found | select-single | get id)
  });

   let update_list = $the_list | update updated { |r| if ($r.id == $selected_id and $r.done == false) { date now } else {$r.updated} } | update done { |r| if ($r.id == $selected_id) { true } else {$r.done} };
   $update_list | to yaml | save $"($todo_path)" -f
   return ($update_list | where id == $selected_id | full-fields true | export-return );
}

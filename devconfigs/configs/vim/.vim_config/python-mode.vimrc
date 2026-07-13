" Load pylint code plugin
let g:pymode_lint = 1

"" Skip errors and warnings
"E.g. 'E501,W002', 'E2,W' (Skip all Warnings and Errors startswith E2) and
"etc
"let g:pymode_lint_ignore = "E501"

" " Select errors and warnings
" " E.g. "E4,W"
" let g:pymode_lint_select = ""

" " Run linter on the fly
" let g:pymode_lint_onfly = 0

"Switch pylint, pyflakes, pep8, mccabe code-checkers
"Can have multiply values "pep8,pyflakes,mcccabe"
let g:pymode_lint_checker = "pep8,pyflakes,mccabe"

" " Pylint configuration file
" " If file not found use 'pylintrc' from python-mode plugin directory
let g:pymode_lint_config = "$HOME/.pylintrc"

" " Check code every save
let g:pymode_lint_write = 1

"Auto open cwindow if errors be finded
let g:pymode_lint_cwindow = 0

"Show error message if cursor placed at the error line
let g:pymode_lint_message = 1

"Auto jump on first error
let g:pymode_lint_jump = 0

" Hold cursor in current window
" when quickfix is open
let g:pymode_lint_hold = 0

" Place error signs
let g:pymode_lint_signs = 1

" Maximum allowed mccabe complexity
let g:pymode_lint_mccabe_complexity = 8

" Minimal height of pylint error window
let g:pymode_lint_minheight = 3

" Maximal height of pylint error window
let g:pymode_lint_maxheight = 6

" Auto create and open ropeproject
let g:pymode_rope_auto_project = 1

" Enable autoimport
let g:pymode_rope_enable_autoimport = 1

" Auto generate global cache
let g:pymode_rope_autoimport_generate = 1

let g:pymode_rope_autoimport_generate = 1

let g:pymode_rope_autoimport_underlineds = 0

let g:pymode_rope_codeassist_maxfixes = 10

let g:pymode_rope_sorted_completions = 0

let g:pymode_rope_extended_complete = 0

let g:pymode_rope_autoimport_modules = ["os","sys","shutil","datetime"]

let g:pymode_rope_confirm_saving = 1

"let g:pymode_rope_global_prefix = "<C-x>p"
"let g:pymode_rope_local_prefix = "<C-c>r"

let g:pymode_rope_vim_completion = 0

let g:pymode_rope_guess_project = 0

let g:pymode_rope_goto_def_newwin = ""

let g:pymode_rope_always_show_complete_menu = 0

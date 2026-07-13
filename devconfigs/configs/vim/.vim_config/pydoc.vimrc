"If you want to use the script and pydoc is not in your PATH, just put a line like this in your .vimrc:
"let g:pydoc_cmd = '/usr/bin/pydoc'
"or more portable
"let g:pydoc_cmd = 'python -m pydoc'
"If you want to open pydoc files in vertical splits or tabs, give the appropriate command in your .vimrc with:
"let g:pydoc_open_cmd = 'vsplit'
"or
let g:pydoc_open_cmd = 'tabnew'

"The script will highlight the search term by default. To disable this behaviour put in your .vimrc:
"let g:pydoc_highlight=0

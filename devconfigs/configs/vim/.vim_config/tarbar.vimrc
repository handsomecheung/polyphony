"""""""""""""""""tagbar""""""""""""""""""""""""""""""""
nmap <leader>t :TagbarOpenAutoClose<CR>
nmap <F4> :TagbarToggle<CR>
let g:tagbar_left = 1
"let g:tagbar_autoclose = 1
let g:tagbar_width = 35
let g:tagbar_autofocus = 1
let g:tagbar_sort = 0
let g:tagbar_compact = 1

"使gvim能自动使用ctags的。
let Tlist_Ctags_Cmd = '/usr/bin/ctags'
"---------------------------------------------------------------------

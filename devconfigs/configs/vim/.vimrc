set nocompatible
"set foldmethod=indent
"autocmd FileType python setlocal foldmethod=indent
"Expand all code by default
"set indentlevel=99
"set autoindent
set nocompatible
set autochdir
set nopaste
set pastetoggle=<F9>
set fenc=utf-8 "set default encoding
set fencs=utf-8,usc-bom,euc-jp,gb18030,gbk,gb2312,cp936
set nocp "or set nocompatible to disable VI compatibility mode
set number "show line numbers
set ai "or set autoindent, Vim uses auto-alignment, applying the current line's alignment to the next line
set si "or set smartindent, smartly choosing the alignment method based on the above

set shiftwidth=4
set tabstop=4 "set Tab key to 4 spaces
set expandtab
set softtabstop=4

autocmd FileType python setlocal et sta sw=4 sts=4
autocmd FileType ruby setlocal et sta sw=2 sts=2

"set sw=4 "or set shiftwidth to set 4 spaces for indentation levels
"
set mouse=a

set ruler "show the cursor position in the status line at the bottom right
set incsearch "set incremental search, which makes searching smarter
set hlsearch
set showmatch "highlight matching brackets
set matchtime=1 "matching bracket highlight duration (unit: 1/10 s)
"set ignorecase "ignore case during search

set nobackup

syntax on "enable syntax highlighting
filetype on
filetype plugin on

"highlight ErrorMsg term=bold cterm=bold ctermfg=Red ctermbg=White guifg=White guibg=Red
"highlight Error term=reverse cterm=bold ctermfg=Red ctermbg=White guifg=White guibg=Red
highlight SpellBad term=reverse cterm=bold ctermfg=Red ctermbg=White  gui=undercurl guisp=Red

set tags=tags;/
"Swap <c-]> and g<c-]> in Normal and Visual modes
"nnoremap <c-]> g<c-]>
"vnoremap <c-]> g<c-]>
"
"nnoremap g<c-]> <c-]>
"vnoremap g<c-]> <c-]>

"Color scheme
"colorscheme slate
"highlight Pmenu guibg=black gui=bold
"highlight Pmenu ctermbg=lightgray gui=bold

"Set mapleader
let mapleader = ","
let g:mapleader = ","

"Swap <c-]> and g<c-]> in Normal and Visual modes
nnoremap <c-]> g<c-]>
vnoremap <c-]> g<c-]>

nnoremap g<c-]> <c-]>
vnoremap g<c-]> <c-]>

"Show a list for selection when ctags has multiple matches
set cscopequickfix=s-,c-,d-,i-,t-,e-,f-


nmap <leader>nh :nohlsearch<cr>
nmap gn :tabedit<cr>
noremap <leader>y "+y
noremap <leader>p "+p



nmap <C-Y> :update<CR>
vmap <C-Y> <C-C>:update<CR>
imap <C-Y> <C-O>:update<CR>

call pathogen#runtime_append_all_bundles()


map <leader>hcc :call SetColorColumn()<CR>
function! SetColorColumn()
    let col_num = virtcol(".")
    let cc_list = split(&cc, ',')
    if count(cc_list, string(col_num)) <= 0
        execute "set cc+=".col_num
    else
        execute "set cc-=".col_num
    endif
endfunction

" Remove trailing whitespace when writing a buffer, but not for diff files.
" @see http://blog.bs2.to/post/EdwardLee/17961
function! RemoveTrailingWhitespace()
    if &ft != "diff"
        let b:curcol = col(".")
        let b:curline = line(".")
        silent! %s/\s\+$//
        silent! %s/\(\s*\n\)\+\%$//
        call cursor(b:curline, b:curcol)
    endif
endfunction
autocmd BufWritePre * call RemoveTrailingWhitespace()

" Normal Mode, Visual Mode, and Select Mode,
" use <Tab> and <Shift-Tab> to indent
" @see http://c9s.blogspot.com/2007/10/vim-tips.html
nmap <tab> v>
nmap <s-tab> v<
vmap <tab> >gv
vmap <s-tab> <gv


"--------------auto complete-------------------------------------
"Make Vim's autocomplete menu behave like general IDEs (refer to VimTip1228)
set completeopt+=longest

"Automatically close the preview window after leaving Insert mode
autocmd InsertLeave * if pumvisible() == 0|pclose|endif

"Press Enter to select the current item
"inoremap <expr> <CR>       pumvisible() ? "\<C-y>" : "\<CR>"

"Behavior of arrow keys
"inoremap <expr> <Down>     pumvisible() ? "\<C-n>" : "\<Down>"
"inoremap <expr> <Up>       pumvisible() ? "\<C-p>" : "\<Up>"
"inoremap <expr> <PageDown> pumvisible() ? "\<PageDown>\<C-p>\<C-n>" : "\<PageDown>"
"inoremap <expr> <PageUp>   pumvisible() ? "\<PageUp>\<C-p>\<C-n>" : "\<PageUp>"
"
"---------------------------------------------------------------------


"--------------------Quickfix---------------------
nmap <leader>en :cn<cr>
nmap <leader>ep :cp<cr>
nmap <leader>ew :cw 10<cr>
nmap <leader>ec :cclose<cr>
"---------------------------------------------------------------------


"Highlight current cursor line (underline)
"set cursorline
"hi cursorline guibg=NONE gui=underline
"
"
"MySQL
let g:dbext_default_profile_mysql_local = 'type=MYSQL:user=root:passwd=root:dbname=mysql:extra=-t'
let g:dbext_default_profile_mysql_local = 'type=MYSQL:user=root:passwd=root:dbname=ticket_dev:extra=--batch --raw --silent'
let g:dbext_default_profile_mysql_local_DBI = 'type=DBI:user=root:passwd=root:driver=mysql:conn_parms=database=mysql;host=localhost'
let g:dbext_default_profile_mysql_local_ODBC = 'type=ODBC:user=root:passwd=root:dsnname=mysql'



"session
" Auto Session Save/Restore
function GetProjectName()
    " Get the current editing file list, Unix only
    let edit_files = split(system("ps -o command= -p " . getpid()))
    if len(edit_files) >= 2
        let project_path = edit_files[1]
        if project_path[0] != '/'
            let project_path = getcwd() . project_path
        endif
    else
        let project_path = getcwd()
    endif

    return shellescape(substitute(project_path, '[/]', '', 'g'))
endfunction

function SaveSession()
    "NERDTree doesn't support session, so close before saving
    execute ':NERDTreeClose'
    let project_name = GetProjectName()
    execute 'mksession! ~/.vim/sessions/' . project_name
endfunction

function RestoreSession()
    let session_path = expand('~/.vim/sessions/' . GetProjectName())
    if filereadable(session_path)
        execute 'so ' . session_path
        if bufexists(1)
            for l in range(1, bufnr('$'))
                if bufwinnr(l) == -1
                    exec 'sbuffer ' . l
                endif
            endfor
        endif
    endif
    "Make sure the syntax is on
    syntax on
endfunction

"nmap ssa :call SaveSession()
"smap SO :call RestoreSession()
"autocmd VimLeave * call SaveSession()
"autocmd VimEnter * call RestoreSession()

" Persistent undo
set undodir=~/.vim/undodir
set undofile
set undolevels=1000 "maximum number of changes that can be undone
set undoreload=10000 "maximum number lines to save for undo on a buffer reload


source ~/.vim_config/minibufexpl.vimrc
source ~/.vim_config/lookupfile.vimrc
source ~/.vim_config/NerdTree.vimrc
source ~/.vim_config/flake8.vimrc
source ~/.vim_config/pydoc.vimrc
source ~/.vim_config/linemotion.vimrc
source ~/.vim_config/grep.vim.vimrc
source ~/.vim_config/vimpress.vimrc
source ~/.vim_config/powerline.vimrc
source ~/.vim_config/tarbar.vimrc
source ~/.vim_config/python-mode.vimrc
source ~/.vim_config/neocomplcache.vimrc
source ~/.vim_config/fuzzyfind.vimrc
source ~/.vim_config/dwm.vim.vimrc
source ~/.vim_config/easymotion.vimrc

"source ~/.vim_config/taglist.vimrc
"source ~/.vim_config/winManager.vimrc
"source ~/.vim_config/pydiction.vimrc

inoremap <C-g> <ESC>

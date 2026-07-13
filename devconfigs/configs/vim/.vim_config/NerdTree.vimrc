nmap <leader>dd :NERDTreeToggle<cr>
nmap <leader>df :NERDTree<space>
"nmap tk :NERDTreeClose<cr>
"autocmd VimEnter * NERDTree "启动Vim时自动打开nerdtree
"let NERDTreeShowBookmarks=1 "一直显示书签
let NERDTreeChDirMode=2 "打开书签时，自动将Vim的pwd设为打开的目录，如果你的项目有tags文件，你会发现这个命令很有帮助
let NERDChristmasTree=1
let NERDTreeIgnore=['\.pyc$', '\~$']

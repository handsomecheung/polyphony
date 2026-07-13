let Tlist_Show_One_File=1  "不同时显示多个文件的tag，只显示当前文件的
let Tlist_File_Fold_Auto_Close=1
let Tlist_Show_One_File=1
nmap <leader>t :Tlist<cr>
"let Tlist_Use_LEFT_Window=1
let Tlist_Use_Right_Window=1    "在右侧窗口中显示taglist窗口
"let Tlist_Auto_Open=1  "启动vim后，自动打开taglist窗口
" F4: Switch on/off TagList
nnoremap <silent> <F4> :TlistToggle<CR>
"let Tlist_Show_One_File = 1 " Displaying tags for only one file~
let Tlist_Exist_OnlyWindow = 1 " if you are the last, kill yourself
"let Tlist_Use_Right_Window = 1 " split to the right side of the screen
let Tlist_Sort_Type = "order" " sort by order or name
let Tlist_Display_Prototype = 0 " do not show prototypes and not tags in the taglist window.
let Tlist_Compart_Format = 1 " Remove extra information and blank lines from the taglist window.
let Tlist_GainFocus_On_ToggleOpen = 1 " Jump to taglist window on open.
let Tlist_Display_Tag_Scope = 1 " Show tag scope next to the tag name.
"let Tlist_Close_On_Select = 1 " Close the taglist window when a file or tag is selected.
let Tlist_Enable_Fold_Column = 0 " Don't Show the fold indicator column in the taglist window.
let Tlist_WinWidth = 40
" let Tlist_Ctags_Cmd = 'ctags --c++-kinds=+p --fields=+iaS --extra=+q --languages=c++'
" very slow, so I disable this
" let Tlist_Process_File_Always = 1 " To use the :TlistShowTag and the :TlistShowPrototype commands without the taglist window and the taglist menu, you should set this variable to 1.
":TlistShowPrototype [filename] [linenumber]

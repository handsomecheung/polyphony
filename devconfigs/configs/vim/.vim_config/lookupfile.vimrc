"
"-----------------------------------------------------------------------------
"" lookupfile.vim 插件设置
"-----------------------------------------------------------------------------

" lookup file with ignore case
function! LookupFile_IgnoreCaseFunc(pattern)
    let _tags = &tags
    try
        let &tags = eval(g:LookupFile_TagExpr)
        let newpattern = '\c' . a:pattern
        let tags = taglist(newpattern)
    catch
        echohl ErrorMsg | echo "Exception: " . v:exception | echohl NONE
        return ""
    finally
        let &tags = _tags
    endtry

    " Show the matches for what is typed so far.
    let files = map(tags, 'v:val["filename"]')
    return files
endfunction
let g:LookupFile_LookupFunc = 'LookupFile_IgnoreCaseFunc'

let g:LookupFile_MinPatLength = 2               "最少输入2个字符才开始查找
let g:LookupFile_PreserveLastPattern = 0        "不保存上次查找的字符串
let g:LookupFile_PreservePatternHistory = 1     "保存查找历史
let g:LookupFile_AlwaysAcceptFirst = 1          "回车打开第一个匹配项目
let g:LookupFile_AllowNewFiles = 0              "不允许创建不存在的文件
let g:LookupFile_SortMethod = ""                "关闭对搜索结果的字母排序

"if filereadable("/home/ganquan/linux-2.6.34-rc4/filenametags")
""设置tag文件的名字
"let g:LookupFile_TagExpr ='"/home/ganquan/linux-2.6.34-rc4/filenametags"'
"endif

let l_pwd = getcwd()
let index = strridx(l_pwd, '/')
let dir_name = strpart(l_pwd, index + 1)
let filenametags = l_pwd . "/" . "filenametags"

if filereadable(filenametags)
    let g:LookupFile_TagExpr = '"' . filenametags . '"'
else
    let g:LookupFile_TagExpr = '"~/.vim_config/filenametags"'

endif

nmap <silent> <leader>lt :LUTags<cr>
nmap <silent> <leader>lb :LUBufs<cr>
nmap <silent> <leader>lw :LUWalk<cr>
"

"let g:LookupFile_TagExpr = '"/home/box/ticket_dev/src/filenametags"'
"let g:LookupFile_TagExpr = '"/home/box/ticket_telphone/src/filenametags"'
"let g:LookupFile_TagExpr = '"/home/box/ticket_sale_machine/src/filenametags"'
"noremap <leader>ld let g:LookupFile_TagExpr = '"/home/box/ticket_dev/src/filenametags"'
"noremap <leader>la let g:LookupFile_TagExpr = '"/home/box/ticket_agent_channel_system/ticket_agent_channel_system/filenametags"'
"noremap <leader>lt let g:LookupFile_TagExpr = '"/home/box/ticket_telphone/src/filenametags"'
"noremap <leader>ls let g:LookupFile_TagExpr = '"/home/box/ticket_sale_machine/src/filenametags"'

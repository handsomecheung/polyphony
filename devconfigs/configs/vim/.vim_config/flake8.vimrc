" 禁止PyFlakes使用QuickFix，这样在按下<F7>时会调用flake8，而有对于代码编辑时的错误仍能得到有效的提示
let g:pyflakes_use_quickfix = 0

" 如有需要，可设置忽略部分错误
let g:flake8_ignore="E501"

let g:indent_guides_guide_size=1

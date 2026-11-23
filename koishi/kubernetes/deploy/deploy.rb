#!/usr/bin/ruby
# frozen_string_literal: true

require File.join(File.expand_path('lib', __dir__), 'common')
require File.join(File.expand_path('lib', __dir__), 'kubectl')

$stdout.sync = true

def main
  _arguments, options = Common.parse_argv ARGV
  puts "options: #{options}\n\n"

  if !options['file'].nil?
    Kubectl.setup_config(options['file'], options.reject { |k, _v| k == 'file' })
  elsif !options['meta'].nil?
    Kubectl.setup_meta(options['meta'], options.reject { |k, _v| k == 'meta' })
  end
  puts "\nSUCCESS"

  exit 0
end

main if __FILE__ == $PROGRAM_NAME

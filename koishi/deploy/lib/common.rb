# frozen_string_literal: false

require 'json'
require 'English'

module Common
  def self.parse_argv(argv)
    options = argv.join(' ').scan(/--?([^=\s]+)(?:=(\S+))?/).to_h
    arguments = argv.reject { |arg| arg.start_with? '--' }

    [arguments, options]
  end

  def self.get_basename(filename)
    File.basename(filename, '.*')
  end

  def self.run_command(cmd, options = {})
    puts "\n\nrun shell command: \n#{cmd.split(' ')[0..2].join(' ')} ..."

    parse_json = options[:parse_json]
    raise_error = options[:raise_error]

    out = `#{cmd}`
    s = $CHILD_STATUS
    raise [s.exitstatus, "failed to run command. cmd: `#{cmd}`, out: `#{out}`"] if raise_error && !s.success?

    return [s.exitstatus, out] if !parse_json

    if out.to_s.empty?
      [s.exitstatus, {}]
    else
      [s.exitstatus, JSON.parse(out)]
    end
  end

  def self.run_raw_shell(cmd, panic = true)
    puts "run raw shell command: \n#{cmd}"
    status = system(cmd)
    raise "failed to run raw command. cmd: `#{cmd}`" if panic && !status

    status
  end
end

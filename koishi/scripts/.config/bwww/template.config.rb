# frozen_string_literal: true

module BWW
  module Config
    @envs = {
      'BWW_URL' => '',
    }.freeze

    def self.get_envs
      Marshal.load(Marshal.dump(@envs))
    end
  end
end

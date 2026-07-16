function has_phrase(sentence, phrase) {
  return index(" " sentence " ", " " phrase " ") != 0
}

function normalize(sentence) {
  sentence = tolower(sentence)
  gsub(/[^[:alnum:]_]+/, " ", sentence)
  gsub(/[[:space:]]+/, " ", sentence)
  sub(/^ /, "", sentence)
  sub(/ $/, "", sentence)
  return sentence
}

function grants_permission(sentence, padded) {
  padded = " " sentence " "
  if (index(padded, " may ") || index(padded, " can "))
    return 1

  if (sentence ~ /(^| )(allows|permits|supports)( |$)/)
    return 1

  if (sentence ~ /(^| )(allowed|permitted|supported)( |$)/) {
    if (sentence ~ /(^| )(not|never) (explicitly )?(allowed|permitted|supported)( |$)/)
      return 0
    return 1
  }

  return 0
}

function safe_release_prohibition(sentence) {
  return has_phrase(sentence, "never release") ||
         has_phrase(sentence, "do not release") ||
         has_phrase(sentence, "must not release") ||
         has_phrase(sentence, "cannot release") ||
         has_phrase(sentence, "may not release")
}

function unsafe_writer_slot_policy(sentence, unsafe_method) {
  if (!has_phrase(sentence, "writer slot") ||
      (!has_phrase(sentence, "release") && !has_phrase(sentence, "released")))
    return 0

  if (sentence ~ /(^| )exact (writer slot )?token (is|are|was|were) not required( |$)/)
    return 1

  unsafe_method = sentence ~ /(^| )without ((its|the|an) )?exact (writer slot )?token( |$)/ ||
                  sentence ~ /(^| )using only ((its|the) )?slot (name|id|number)( |$)/ ||
                  sentence ~ /(^| )by ((its|the) )?slot (name|id|number)( |$)/

  return unsafe_method && !safe_release_prohibition(sentence)
}

function unsafe_agent_team_policy(sentence) {
  if (!has_phrase(sentence, "agent teams"))
    return 0
  if (!has_phrase(sentence, "recursive sdd") &&
      !has_phrase(sentence, "recursive coordinator"))
    return 0
  return grants_permission(sentence)
}

{
  document = document " " $0
}

END {
  sentence_count = split(document, sentences, /[.!?]+/)
  for (i = 1; i <= sentence_count; i++) {
    sentence = normalize(sentences[i])
    if (policy == "writer" && unsafe_writer_slot_policy(sentence))
      unsafe = 1
    if (policy == "teams" && unsafe_agent_team_policy(sentence))
      unsafe = 1
  }
  exit unsafe
}

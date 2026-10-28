-- Who may open a file outside the module that stored it (the files administration page): the
-- permission the owner module names when it stores the file. Empty — every existing file — means
-- only through its owner module, which decides who may see it. The storage key index serves the
-- sweep that removes bytes no row refers to (a save whose transaction rolled back).
ALTER TABLE files
  ADD COLUMN read_permission VARCHAR(80) NOT NULL DEFAULT '',
  ADD KEY files_storage_key (storage_key);
